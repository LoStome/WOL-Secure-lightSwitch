package sshrunner

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"secure-switch-backend/internal/config"
)

func testSSHSigner(t *testing.T) ssh.Signer {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

func testSSHShutdownEndpoint(t *testing.T, signer ssh.Signer) (net.Listener, *config.Host) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })

	knownHostsPath := filepath.Join(t.TempDir(), "known_hosts")
	line := knownhosts.Line([]string{listener.Addr().String()}, signer.PublicKey()) + "\n"
	if err := os.WriteFile(knownHostsPath, []byte(line), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SSH_KNOWN_HOSTS_FILE", knownHostsPath)
	passwordPath := filepath.Join(t.TempDir(), "ssh_password")
	if err := os.WriteFile(passwordPath, []byte("test-password\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return listener, &config.Host{IP: "127.0.0.1", User: "test", PasswordFile: passwordPath, Cmd: "test-command"}
}
