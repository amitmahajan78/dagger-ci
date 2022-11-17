package main

import (
	"context"
	"fmt"
	"os"

	"dagger.io/dagger"
)

var platformToArch = map[dagger.Platform]string{
	"linux/amd64": "amd64",
	"linux/arm64": "arm64",
}

func main() {
	ctx := context.Background()

	task := os.Args[1]

	if len(os.Args) < 2 {
		fmt.Println("Please pass a task as an argument [ test | push ]")
		os.Exit(1)
	}

	var err error

	switch task {
	case "test":
		err = test(ctx)
	case "push":
		err = push(ctx)
	default:
		fmt.Printf("Unknown task %s\n", task)
		os.Exit(1)
	}

	if err != nil {
		fmt.Printf("failed to run task %s: %+v\n", task, err)
	}
}

func test(ctx context.Context) error {
	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		return err
	}
	defer client.Close()

	src := client.Host().Workdir(dagger.HostWorkdirOpts{
		Exclude: []string{
			"ci/*",
		},
	})

	testoutput := client.Directory()
	cacheKey := "gomods"
	cache := client.CacheVolume(cacheKey)

	// multiplatform tests
	goversions := []string{"1.19"}
	platforms := []dagger.Platform{"linux/arm64"}

	for _, plat := range platforms {
		for _, goversion := range goversions {
			image := fmt.Sprintf("golang:%s", goversion)
			builder := client.Container(dagger.ContainerOpts{Platform: plat}).
				From(image).
				WithMountedDirectory("/src", src).
				WithWorkdir("/src").
				WithMountedCache("/cache", cache).
				WithEnvVariable("GOMODCACHE", "/cache").
				Exec(dagger.ContainerExecOpts{
					Args: []string{"go", "test"},
				})

			// Get Command Output
			outputfile := fmt.Sprintf("output/%s/%s.out", string(plat), goversion)
			testoutput = testoutput.WithFile(
				outputfile,
				builder.Stdout(),
			)
		}
	}

	_, err = testoutput.Export(ctx, ".")
	if err != nil {
		return err
	}
	fmt.Println("Successfully Build and Test")
	return nil
}

func push(ctx context.Context) error {
	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		return err
	}
	defer client.Close()

	// get project dir
	src := client.Host().Workdir()

	variants := make([]*dagger.Container, 0, len(platformToArch))
	for platform, arch := range platformToArch {
		// assemble golang build
		builder := client.Container().
			From("golang:latest").
			WithMountedDirectory("/src", src).
			WithWorkdir("/src").
			WithEnvVariable("CGO_ENABLED", "0").
			WithEnvVariable("GOOS", "linux").
			WithEnvVariable("GOARCH", arch).
			Exec(dagger.ContainerExecOpts{
				Args: []string{"go", "build", "-o", "/src/dagger-api"},
			})

		// Build container on production base with build artifact
		base := client.Container(dagger.ContainerOpts{Platform: platform}).
			From("alpine")
		// copy build artifact from builder image
		base = base.WithFS(
			base.FS().WithFile("/bin/dagger-api",
				builder.File("/src/dagger-api"),
			)).
			WithEntrypoint([]string{"/bin/dagger-api"})
		// add built container to container variants
		variants = append(variants, base)
	}
	// Publish all images
	addr, err := client.Container().Publish(ctx,
		"amitmahajan/dagger-ci:1.1",
		dagger.ContainerPublishOpts{
			PlatformVariants: variants,
		})
	if err != nil {
		return err
	}

	fmt.Printf("Successfully Published in Docker Hub: %s", addr)
	return nil
}

func apply() error {
	ctx := context.Background()

	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		return fmt.Errorf("Error connecting to Dagger Engine: %s", err)
	}

	defer client.Close()

	src := client.Host().Workdir()
	if err != nil {
		return fmt.Errorf("Error getting reference to host directory: %s", err)
	}

	golang := client.Container().From("golang:latest")
	golang = golang.WithMountedDirectory("/src", src).
		WithWorkdir("/src").
		WithEnvVariable("CGO_ENABLED", "0")

	golang = golang.Exec(
		dagger.ContainerExecOpts{
			Args: []string{"go", "build", "-o", "build/"},
		},
	)

	path := "build/"
	build := golang.Directory(path)

	_, err = client.Container().From("alpine:latest").
		WithMountedDirectory("/tmp", build).
		Exec(dagger.ContainerExecOpts{
			Args: []string{"cp", "/tmp/dagger-ci", "/bin/dagger-ci"},
		}).
		WithEntrypoint([]string{"/bin/dagger-ci"}).
		Publish(ctx, "amitmahajan/dagger-ci:latest")

	if err != nil {
		return fmt.Errorf("Error creating and pushing container: %s", err)
	}
	return nil
}
