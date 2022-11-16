package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
)

func main() {
	http.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		fmt.Printf("Request came from %s\n", r.RemoteAddr)
		fmt.Fprint(rw, "<h1>Hello, Welcome to Dagger, CI example, Dagger is a programmable CI/CD engine that runs your pipelines in containers.</h1>")
	})

	err := http.ListenAndServe(":9090", nil)
	if errors.Is(err, http.ErrServerClosed) {
		fmt.Printf("server closed\n")
	} else if err != nil {
		fmt.Printf("error starting server: %s\n", err)
		os.Exit(1)
	}
}

func getInfo() string {
	return "this is dagger ci demo with feature 1"
}
