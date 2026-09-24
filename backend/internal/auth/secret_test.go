package auth

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLoadJWTSecretRejectsInvalidConfiguration(t *testing.T) {
	t.Setenv("INITIALIZE_DATA", "")
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

			if _, err := LoadJWTSecret(); err == nil {
				t.Fatal("LoadJWTSecret() succeeded with an invalid secret")
			}
		})
	}
}

func TestLoadJWTSecretFromEnvironment(t *testing.T) {
	t.Setenv("INITIALIZE_DATA", "true")
	want := strings.Repeat("a", MinimumJWTSecretLength)
	t.Setenv("JWT_SECRET_FILE", "")
	t.Setenv("JWT_SECRET", "  "+want+"  ")

	got, err := LoadJWTSecret()
	if err != nil {
		t.Fatalf("LoadJWTSecret() returned an error: %v", err)
	}
	if string(got) != want {
		t.Fatalf("LoadJWTSecret() = %q, want %q", got, want)
	}
}

func TestLoadJWTSecretFromFile(t *testing.T) {
	want := strings.Repeat("b", MinimumJWTSecretLength)
	secretFile := filepath.Join(t.TempDir(), "jwt_secret")
	if err := os.WriteFile(secretFile, []byte(want+"\n"), 0o600); err != nil {
		t.Fatalf("write test secret: %v", err)
	}
	t.Setenv("JWT_SECRET_FILE", secretFile)
	t.Setenv("JWT_SECRET", "")

	got, err := LoadJWTSecret()
	if err != nil {
		t.Fatalf("LoadJWTSecret() returned an error: %v", err)
	}
	if string(got) != want {
		t.Fatalf("LoadJWTSecret() = %q, want %q", got, want)
	}
}

func TestLoadJWTSecretFileTakesPrecedence(t *testing.T) {
	want := strings.Repeat("c", MinimumJWTSecretLength)
	secretFile := filepath.Join(t.TempDir(), "jwt_secret")
	if err := os.WriteFile(secretFile, []byte(want), 0o600); err != nil {
		t.Fatalf("write test secret: %v", err)
	}
	t.Setenv("JWT_SECRET_FILE", secretFile)
	t.Setenv("JWT_SECRET", "short-secret")

	got, err := LoadJWTSecret()
	if err != nil {
		t.Fatalf("LoadJWTSecret() returned an error: %v", err)
	}
	if string(got) != want {
		t.Fatalf("LoadJWTSecret() = %q, want %q", got, want)
	}
}

func TestLoadJWTSecretDoesNotFallBackWhenFileIsMissing(t *testing.T) {
	t.Setenv("INITIALIZE_DATA", "true")
	t.Setenv("JWT_SECRET_FILE", filepath.Join(t.TempDir(), "missing"))
	t.Setenv("JWT_SECRET", strings.Repeat("d", MinimumJWTSecretLength))

	if _, err := LoadJWTSecret(); err == nil {
		t.Fatal("LoadJWTSecret() succeeded with a missing configured secret file")
	}
}

func TestGeneratedJWTSecretPersistsAndInvalidFileIsNotReplaced(t *testing.T) {
	workDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(workDir, "data"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(workDir)
	t.Setenv("INITIALIZE_DATA", "true")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("JWT_SECRET_FILE", "")

	first, err := LoadJWTSecret()
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 64 {
		t.Fatalf("generated secret length = %d, want 64", len(first))
	}
	path := filepath.Join("data", "jwt_secret")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("secret permissions = %04o, want 0600", info.Mode().Perm())
	}
	second, err := LoadJWTSecret()
	if err != nil || string(second) != string(first) {
		t.Fatalf("second load changed secret: %v", err)
	}
	if err := os.WriteFile(path, []byte("short-secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadJWTSecret(); err == nil {
		t.Fatal("accepted invalid existing secret")
	}
	contents, err := os.ReadFile(path)
	if err != nil || string(contents) != "short-secret" {
		t.Fatalf("invalid existing secret was changed: %q, %v", contents, err)
	}
}
