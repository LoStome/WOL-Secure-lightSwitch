package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestHostNetworkBindsApplicationToLoopback(t *testing.T) {
	composePath := filepath.Join("..", "docker-compose.yml")
	contents, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("read %s: %v", composePath, err)
	}

	var config struct {
		Services map[string]struct {
			NetworkMode string   `yaml:"network_mode"`
			Environment []string `yaml:"environment"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(contents, &config); err != nil {
		t.Fatalf("parse %s: %v", composePath, err)
	}

	service, ok := config.Services["wol-switch"]
	if !ok {
		t.Fatal("docker-compose.yml does not define the wol-switch service")
	}
	if service.NetworkMode != "host" {
		t.Skip("loopback bind invariant applies only when host networking is enabled")
	}

	for _, entry := range service.Environment {
		if entry == "BIND_ADDRESS=127.0.0.1" {
			return
		}
	}

	t.Fatal("wol-switch must set BIND_ADDRESS=127.0.0.1 when using host networking")
}

func TestDockerfileRunsRuntimeAsDedicatedUser(t *testing.T) {
	dockerfile, err := os.ReadFile("../Dockerfile")
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}

	contents := string(dockerfile)
	for _, required := range []string{
		"addgroup -S wol",
		"adduser -S -G wol -h /app -s /sbin/nologin wol",
		"mkdir -p /app/data",
		"chown -R wol:wol /app",
		"COPY --chown=wol:wol --from=backend-builder /app/backend/wol-server /app/wol-server",
		"COPY --chown=wol:wol --from=frontend-builder /app/frontend/dist /app/frontend/dist",
		"USER wol",
	} {
		if !strings.Contains(contents, required) {
			t.Errorf("Dockerfile must contain %q", required)
		}
	}
}

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
