package main

import (
	"os"
	"strings"
	"testing"
)

func TestDockerignoreExcludesSensitiveAndLocalBuildContext(t *testing.T) {
	dockerignore, err := os.ReadFile("../.dockerignore")
	if err != nil {
		t.Fatalf("read .dockerignore: %v", err)
	}

	contents := string(dockerignore)
	for _, required := range []string{
		".git",
		"data/",
		"frontend/node_modules/",
		"frontend/dist/",
		"*.bak",
		"*.exe",
		".env",
		".env.*",
		"docs/",
	} {
		if !strings.Contains(contents, required) {
			t.Errorf(".dockerignore must exclude %q", required)
		}
	}
}
