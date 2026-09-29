package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureInitialHostsOnlyCreatesMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hosts.yaml")
	if err := EnsureInitialHosts(path); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil || string(contents) != "[]\n" {
		t.Fatalf("initial hosts = %q, %v", contents, err)
	}
	if hosts, err := ParseHosts(contents); err != nil || hosts == nil || len(hosts) != 0 {
		t.Fatalf("initial hosts invalid: %+v, %v", hosts, err)
	}
	invalid := []byte("invalid: [yaml")
	if err := os.WriteFile(path, invalid, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := EnsureInitialHosts(path); err != nil {
		t.Fatal(err)
	}
	contents, err = os.ReadFile(path)
	if err != nil || string(contents) != string(invalid) {
		t.Fatalf("existing hosts were changed: %q, %v", contents, err)
	}
}
