package sshrunner

import (
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"secure-switch-backend/internal/config"
)

func TestRemoteShutdownReturnsCommandError(t *testing.T) {
	const command = "test-command"

	passwordPath := filepath.Join(t.TempDir(), "ssh_password")
	if err := os.WriteFile(passwordPath, []byte("test-password\n"), 0600); err != nil {
		t.Fatal(err)
	}

	serverKey := testSSHSigner(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	t.Cleanup(func() {
		listener.Close()
		<-done
	})

	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(5 * time.Second))

		config := &ssh.ServerConfig{
			PasswordCallback: func(ssh.ConnMetadata, []byte) (*ssh.Permissions, error) {
				return nil, nil
			},
		}
		config.AddHostKey(serverKey)
		server, channels, requests, err := ssh.NewServerConn(conn, config)
		if err != nil {
			return
		}
		defer server.Close()
		go ssh.DiscardRequests(requests)
		for newChannel := range channels {
			channel, channelRequests, err := newChannel.Accept()
			if err != nil {
				return
			}
			for request := range channelRequests {
				if request.Type != "exec" {
					request.Reply(false, nil)
					continue
				}
				request.Reply(true, nil)
				channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{1}))
				channel.Close()
				return
			}
		}
	}()

	knownHostsPath := filepath.Join(t.TempDir(), "known_hosts")
	knownHostsLine := knownhosts.Line([]string{listener.Addr().String()}, serverKey.PublicKey()) + "\n"
	if err := os.WriteFile(knownHostsPath, []byte(knownHostsLine), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SSH_KNOWN_HOSTS_FILE", knownHostsPath)

	err = remoteShutdown(&config.Host{
		IP:           "127.0.0.1",
		User:         "test",
		PasswordFile: passwordPath,
		Cmd:          command,
	}, listener.Addr().String())
	if err == nil {
		t.Fatal("remote shutdown succeeded after command failure")
	}
	if strings.Contains(err.Error(), command) || strings.Contains(err.Error(), "remote-secret") {
		t.Fatalf("command error exposed sensitive details: %v", err)
	}
}

func TestRemoteShutdownHandshakeTimeout(t *testing.T) {
	signer := testSSHSigner(t)
	listener, host := testSSHShutdownEndpoint(t, signer)
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(8 * time.Second))
		io.Copy(io.Discard, conn) // Accept TCP but never send an SSH banner.
	}()
	t.Cleanup(func() {
		listener.Close()
		<-done
	})

	start := time.Now()
	err := remoteShutdown(host, listener.Addr().String())
	if err == nil || err.Error() != "SSH operation timed out" || time.Since(start) > 7*time.Second {
		t.Fatalf("stalled SSH handshake: err=%v, elapsed=%v; want timeout within 7s", err, time.Since(start))
	}
}

func TestRemoteShutdownCommandTimeoutAndOutputLimit(t *testing.T) {
	for _, scenario := range []string{"stalled command", "excessive output"} {
		t.Run(scenario, func(t *testing.T) {
			signer := testSSHSigner(t)
			listener, host := testSSHShutdownEndpoint(t, signer)
			done := make(chan struct{})
			go func() {
				defer close(done)
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				defer conn.Close()
				conn.SetDeadline(time.Now().Add(8 * time.Second))
				config := &ssh.ServerConfig{PasswordCallback: func(ssh.ConnMetadata, []byte) (*ssh.Permissions, error) { return nil, nil }}
				config.AddHostKey(signer)
				server, channels, requests, err := ssh.NewServerConn(conn, config)
				if err != nil {
					return
				}
				defer server.Close()
				go ssh.DiscardRequests(requests)
				for newChannel := range channels {
					channel, channelRequests, err := newChannel.Accept()
					if err != nil {
						return
					}
					for request := range channelRequests {
						if request.Type != "exec" {
							request.Reply(false, nil)
							continue
						}
						request.Reply(true, nil)
						if scenario == "stalled command" {
							server.Wait()
						} else {
							_, _ = channel.Write(make([]byte, 128*1024))
							channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
						}
						channel.Close()
						return
					}
				}
			}()
			t.Cleanup(func() {
				listener.Close()
				<-done
			})

			start := time.Now()
			err := remoteShutdown(host, listener.Addr().String())
			wantError := "SSH operation timed out"
			if scenario == "excessive output" {
				wantError = "SSH output limit exceeded"
			}
			if err == nil || err.Error() != wantError {
				t.Fatalf("shutdown error = %v, want %q", err, wantError)
			}
			if scenario == "stalled command" && time.Since(start) > 7*time.Second {
				t.Fatalf("stalled command took %v; want timeout within 7s", time.Since(start))
			}
		})
	}
}

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
