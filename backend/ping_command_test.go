package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if os.Getenv("REL04_FAKE_PING") == "1" {
		time.Sleep(6 * time.Second)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestPingCommandHasBoundedLifetime(t *testing.T) {
	if os.Getenv("REL04_FAKE_PING") == "1" {
		t.Fatal("fake ping must run in a child process")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	fakeName := "ping"
	if runtime.GOOS == "windows" {
		fakeName += ".exe"
	}
	fakePing := filepath.Join(t.TempDir(), fakeName)
	source, err := os.Open(executable)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	target, err := os.Create(fakePing)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(target, source); err != nil {
		t.Fatal(err)
	}
	if err := target.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(fakePing, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Dir(fakePing)+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("REL04_FAKE_PING", "1")
	resolved, err := exec.LookPath("ping")
	if err != nil || !strings.EqualFold(resolved, fakePing) {
		t.Fatalf("fake ping not selected: path=%q, error=%v", resolved, err)
	}
	started := time.Now()
	if IsOnline("192.0.2.1") {
		t.Fatal("a ping that never responds was reported online")
	}
	if elapsed := time.Since(started); elapsed > 4*time.Second {
		t.Fatalf("ping command blocked for %s; want cancellation before 4s", elapsed)
	}
}
