package main

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestGetInfo(t *testing.T) {
	g := getInfo()
	should := "this is dagger ci demo with feature 1"

	assert.Equal(t, should, g)
}
