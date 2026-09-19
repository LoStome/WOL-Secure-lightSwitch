package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
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

func TestRemoteShutdownHostKeyVerification(t *testing.T) {
	serverKey := testSSHSigner(t)
	otherKey := testSSHSigner(t)
	for _, scenario := range []string{"unknown", "changed", "trusted", "missing", "malformed"} {
		t.Run(scenario, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			var authentications, commands atomic.Int32
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
						authentications.Add(1)
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
					channel, requests, err := newChannel.Accept()
					if err != nil {
						return
					}
					for request := range requests {
						if request.Type != "exec" {
							request.Reply(false, nil)
							continue
						}
						commands.Add(1)
						request.Reply(true, nil)
						channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
						channel.Close()
						return
					}
				}
			}()

			path := filepath.Join(t.TempDir(), "known_hosts")
			content := ""
			switch scenario {
			case "trusted":
				content = knownhosts.Line([]string{listener.Addr().String()}, serverKey.PublicKey()) + "\n"
			case "changed":
				content = knownhosts.Line([]string{listener.Addr().String()}, otherKey.PublicKey()) + "\n"
			case "malformed":
				content = "invalid known hosts entry\n"
			}
			if scenario != "missing" {
				if err := os.WriteFile(path, []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("SSH_KNOWN_HOSTS_FILE", path)
			err = remoteShutdown(&Host{IP: "127.0.0.1", User: "test", Password: "test", Cmd: "test-command"}, listener.Addr().String())
			listener.Close()
			<-done
			if scenario == "trusted" {
				if err != nil || authentications.Load() != 1 || commands.Load() != 1 {
					t.Fatalf("trusted host: err=%v, authentications=%d, commands=%d", err, authentications.Load(), commands.Load())
				}
			} else {
				if err == nil {
					t.Error("untrusted host accepted; want an error")
				}
				if authentications.Load() != 0 || commands.Load() != 0 {
					t.Errorf("untrusted host received authentication or command: authentications=%d, commands=%d", authentications.Load(), commands.Load())
				}
			}
		})
	}
}
