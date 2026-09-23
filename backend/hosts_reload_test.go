package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadHostsKeepsLastValidSnapshotDuringPartialSave(t *testing.T) {
	workDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(workDir, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(workDir)
	path := filepath.Join("data", "hosts.yaml")
	if err := os.WriteFile(path, []byte("- id: host-one\n  name: Host One\n  mac: AA:BB:CC:DD:EE:FF\n  ip: 192.0.2.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	initial, err := LoadHosts()
	if err != nil || len(initial) != 1 || initial[0].ID != "host-one" {
		t.Fatalf("initial hosts = %+v, err = %v", initial, err)
	}
	if err := os.WriteFile(path, []byte("- id: [partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	hosts, err := LoadHosts()
	if err != nil || len(hosts) != 1 || hosts[0].ID != "host-one" {
		t.Fatalf("hosts after partial save = %+v, err = %v; want last valid host", hosts, err)
	}
	if err := os.WriteFile(path, []byte("- id: host-two\n  name: Host Two\n  mac: 00:11:22:33:44:55\n  ip: 192.0.2.22\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	hosts, err = LoadHosts()
	if err != nil || len(hosts) != 1 || hosts[0].ID != "host-two" {
		t.Fatalf("hosts after completed save = %+v, err = %v; want new host", hosts, err)
	}
}

func TestLoadHostsRejectsInvalidInitialConfiguration(t *testing.T) {
	workDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(workDir, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(workDir)
	if err := os.WriteFile(filepath.Join("data", "hosts.yaml"), []byte("- id: [partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	if hosts, err := LoadHosts(); err == nil || hosts != nil {
		t.Fatalf("invalid initial config = %+v, err = %v; want error and no hosts", hosts, err)
	}
}

func TestLoadHostsReturnsIndependentCopies(t *testing.T) {
	workDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(workDir, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(workDir)
	config := "- id: host-one\n  name: Host One\n  mac: AA:BB:CC:DD:EE:FF\n  skip_interfaces: [Loopback]\n"
	if err := os.WriteFile(filepath.Join("data", "hosts.yaml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	hosts, err := LoadHosts()
	if err != nil {
		t.Fatal(err)
	}
	hosts[0].Name = "changed"
	hosts[0].SkipInterfaces[0] = "changed"
	hosts, err = LoadHosts()
	if err != nil || hosts[0].Name != "Host One" || hosts[0].SkipInterfaces[0] != "Loopback" {
		t.Fatalf("snapshot changed through caller: %+v, err = %v", hosts, err)
	}
}

func TestParseHostsRejectsInvalidFields(t *testing.T) {
	valid := "- id: host-one\n  name: Host One\n  mac: AA:BB:CC:DD:EE:FF\n  ip: host.example\n"
	tests := map[string]string{
		"empty file":             "",
		"null document":          "null\n",
		"duplicate ID":           valid + valid,
		"missing name":           "- id: host-one\n  mac: AA:BB:CC:DD:EE:FF\n",
		"invalid MAC":            "- id: host-one\n  name: Host One\n  mac: invalid\n",
		"invalid IP":             "- id: host-one\n  name: Host One\n  mac: AA:BB:CC:DD:EE:FF\n  ip: bad host\n",
		"invalid numeric IP":     "- id: host-one\n  name: Host One\n  mac: AA:BB:CC:DD:EE:FF\n  ip: 999.999.999.999\n",
		"negative ping interval": valid + "  ping_interval: -1\n",
		"both SSH credentials":   valid + "  key_path: key\n  password_file: password\n",
		"unknown field":          valid + "  unexpected: value\n",
	}
	for name, config := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := parseHosts([]byte(config)); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
	if hosts, err := parseHosts([]byte(valid)); err != nil || len(hosts) != 1 {
		t.Fatalf("valid configuration = %+v, err = %v", hosts, err)
	}
}
