package main

import (
	"os"
	"strings"
	"testing"
)

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
