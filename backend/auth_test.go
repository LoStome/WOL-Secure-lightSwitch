package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadJWTSecretRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		secret string
	}{
		{name: "missing"},
		{name: "too short", secret: "short-secret"},
		{name: "old fallback", secret: "default-insecure-secret-change-me"},
		{name: "compose placeholder", secret: "your_very_long_and_random_jwt_secret"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("JWT_SECRET_FILE", "")
			t.Setenv("JWT_SECRET", test.secret)

			if _, err := loadJWTSecret(); err == nil {
				t.Fatal("loadJWTSecret() succeeded with an invalid secret")
			}
		})
	}
}

func TestLoadJWTSecretFromEnvironment(t *testing.T) {
	want := strings.Repeat("a", minimumJWTSecretLength)
	t.Setenv("JWT_SECRET_FILE", "")
	t.Setenv("JWT_SECRET", "  "+want+"  ")

	got, err := loadJWTSecret()
	if err != nil {
		t.Fatalf("loadJWTSecret() returned an error: %v", err)
	}
	if string(got) != want {
		t.Fatalf("loadJWTSecret() = %q, want %q", got, want)
	}
}

func TestLoadJWTSecretFromFile(t *testing.T) {
	want := strings.Repeat("b", minimumJWTSecretLength)
	secretFile := filepath.Join(t.TempDir(), "jwt_secret")
	if err := os.WriteFile(secretFile, []byte(want+"\n"), 0o600); err != nil {
		t.Fatalf("write test secret: %v", err)
	}
	t.Setenv("JWT_SECRET_FILE", secretFile)
	t.Setenv("JWT_SECRET", "")

	got, err := loadJWTSecret()
	if err != nil {
		t.Fatalf("loadJWTSecret() returned an error: %v", err)
	}
	if string(got) != want {
		t.Fatalf("loadJWTSecret() = %q, want %q", got, want)
	}
}

func TestLoadJWTSecretFileTakesPrecedence(t *testing.T) {
	want := strings.Repeat("c", minimumJWTSecretLength)
	secretFile := filepath.Join(t.TempDir(), "jwt_secret")
	if err := os.WriteFile(secretFile, []byte(want), 0o600); err != nil {
		t.Fatalf("write test secret: %v", err)
	}
	t.Setenv("JWT_SECRET_FILE", secretFile)
	t.Setenv("JWT_SECRET", "short-secret")

	got, err := loadJWTSecret()
	if err != nil {
		t.Fatalf("loadJWTSecret() returned an error: %v", err)
	}
	if string(got) != want {
		t.Fatalf("loadJWTSecret() = %q, want %q", got, want)
	}
}

func TestLoadJWTSecretDoesNotFallBackWhenFileIsMissing(t *testing.T) {
	t.Setenv("JWT_SECRET_FILE", filepath.Join(t.TempDir(), "missing"))
	t.Setenv("JWT_SECRET", strings.Repeat("d", minimumJWTSecretLength))

	if _, err := loadJWTSecret(); err == nil {
		t.Fatal("loadJWTSecret() succeeded with a missing configured secret file")
	}
}
