package sshrunner

import (
	"path/filepath"
	"secure-switch-backend/internal/config"
	"strings"
	"testing"
)

func TestRemoteShutdownDoesNotExposePasswordFilePath(t *testing.T) {
	passwordPath := filepath.Join(t.TempDir(), "internal-password")
	err := remoteShutdown(
		&config.Host{IP: "192.0.2.10", User: "test", PasswordFile: passwordPath},
		"192.0.2.10:22",
	)
	if err == nil {
		t.Fatal("remoteShutdown succeeded with a missing password file")
	}
	if strings.Contains(err.Error(), passwordPath) {
		t.Fatalf("remoteShutdown exposed the password file path: %v", err)
	}
}
