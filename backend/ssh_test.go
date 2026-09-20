package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"gopkg.in/yaml.v3"
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
			passwordPath := filepath.Join(t.TempDir(), "ssh_password")
			if err := os.WriteFile(passwordPath, []byte("test\n"), 0600); err != nil {
				t.Fatal(err)
			}
			err = remoteShutdown(&Host{IP: "127.0.0.1", User: "test", PasswordFile: passwordPath, Cmd: "test-command"}, listener.Addr().String())
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

func TestRemoteShutdownReadsPasswordFromFile(t *testing.T) {
	const password = "dedicated-test-password"

	passwordPath := filepath.Join(t.TempDir(), "ssh_password")
	if err := os.WriteFile(passwordPath, []byte(password+"\n"), 0600); err != nil {
		t.Fatal(err)
	}

	hostYAML, err := yaml.Marshal(struct {
		IP           string `yaml:"ip"`
		User         string `yaml:"user"`
		PasswordFile string `yaml:"password_file"`
		Cmd          string `yaml:"cmd"`
	}{
		IP:           "127.0.0.1",
		User:         "test",
		PasswordFile: passwordPath,
		Cmd:          "test-command",
	})
	if err != nil {
		t.Fatal(err)
	}
	var host Host
	if err := yaml.Unmarshal(hostYAML, &host); err != nil {
		t.Fatal(err)
	}

	serverKey := testSSHSigner(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	receivedPassword := make(chan string, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(5 * time.Second))

		config := &ssh.ServerConfig{
			PasswordCallback: func(_ ssh.ConnMetadata, supplied []byte) (*ssh.Permissions, error) {
				receivedPassword <- string(supplied)
				if string(supplied) != password {
					return nil, fmt.Errorf("unexpected password")
				}
				return nil, nil
			},
		}
		config.AddHostKey(serverKey)
		server, channels, requests, handshakeErr := ssh.NewServerConn(conn, config)
		if handshakeErr != nil {
			return
		}
		defer server.Close()
		go ssh.DiscardRequests(requests)
		for newChannel := range channels {
			channel, channelRequests, channelErr := newChannel.Accept()
			if channelErr != nil {
				return
			}
			for request := range channelRequests {
				if request.Type != "exec" {
					request.Reply(false, nil)
					continue
				}
				request.Reply(true, nil)
				channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
				channel.Close()
				return
			}
		}
	}()

	knownHostsPath := filepath.Join(t.TempDir(), "known_hosts")
	knownHostsLine := knownhosts.Line([]string{listener.Addr().String()}, serverKey.PublicKey()) + "\n"
	if err := os.WriteFile(knownHostsPath, []byte(knownHostsLine), 0600); err != nil {
		listener.Close()
		<-done
		t.Fatal(err)
	}
	t.Setenv("SSH_KNOWN_HOSTS_FILE", knownHostsPath)

	err = remoteShutdown(&host, listener.Addr().String())
	listener.Close()
	<-done
	if err != nil {
		t.Fatalf("remote shutdown with password file failed: %v", err)
	}
	select {
	case supplied := <-receivedPassword:
		if supplied != password {
			t.Fatalf("server received password %q, want %q", supplied, password)
		}
	default:
		t.Fatal("server did not receive password authentication")
	}
}
