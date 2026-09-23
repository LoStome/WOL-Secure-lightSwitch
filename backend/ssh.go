package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

const sshOperationTimeout = 5 * time.Second
const sshOutputLimit = 64 * 1024

type sshOutputCounter struct {
	mu       sync.Mutex
	bytes    int
	exceeded bool
}

func (o *sshOutputCounter) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(p) > sshOutputLimit-o.bytes {
		o.exceeded = true
		o.bytes = sshOutputLimit
	} else {
		o.bytes += len(p)
	}
	return len(p), nil
}

func (o *sshOutputCounter) Exceeded() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.exceeded
}

func RemoteShutdown(h *Host) error {
	return remoteShutdown(h, h.IP+":22")
}

func remoteShutdown(h *Host, address string) error {
	var authMethods []ssh.AuthMethod

	user := h.User
	passwordFile := h.PasswordFile
	keyPath := h.KeyPath
	command := h.Cmd

	//default shutdown command if not provided
	if command == "" {
		command = "sudo -n /usr/sbin/poweroff"
	}

	if keyPath != "" && passwordFile != "" {
		return errors.New("invalid SSH authentication configuration")
	}

	if keyPath != "" {

		key, err := os.ReadFile(keyPath)
		if err != nil {
			return errors.New("SSH key unavailable")
		}

		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return errors.New("SSH key invalid")
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))

	} else if passwordFile != "" {
		passwordBytes, err := os.ReadFile(passwordFile)
		if err != nil {
			return errors.New("SSH password configuration unavailable")
		}
		password := strings.TrimRight(string(passwordBytes), "\r\n")
		if password == "" {
			return errors.New("SSH password configuration is empty")
		}

		authMethods = append(authMethods, ssh.Password(password))
	} else {
		return errors.New("SSH authentication is not configured")
	}

	// Provision trusted host keys out of band; never accept new keys automatically.
	// Override the path with SSH_KNOWN_HOSTS_FILE for deployments outside Docker.
	knownHostsPath := os.Getenv("SSH_KNOWN_HOSTS_FILE")
	if knownHostsPath == "" {
		knownHostsPath = "data/.ssh/known_hosts"
	}
	hostKeyCallback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return errors.New("SSH host key configuration unavailable")
	}

	config := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
	}

	ctx, cancel := context.WithTimeout(context.Background(), sshOperationTimeout)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return errors.New("SSH operation timed out")
		}
		return errors.New("SSH connection failed")
	}
	defer conn.Close()
	deadline, _ := ctx.Deadline()
	if err := conn.SetDeadline(deadline); err != nil {
		return errors.New("SSH connection deadline failed")
	}
	sshConn, channels, requests, err := ssh.NewClientConn(conn, address, config)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return errors.New("SSH operation timed out")
		}
		return errors.New("SSH connection failed")
	}
	client := ssh.NewClient(sshConn, channels, requests)
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return errors.New("SSH operation timed out")
		}
		return errors.New("SSH session failed")
	}
	defer session.Close()

	// Count output without retaining remote data or letting it grow in memory.
	output := &sshOutputCounter{}
	session.Stdout = output
	session.Stderr = output
	err = session.Run(command)
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return errors.New("SSH operation timed out")
	}
	if output.Exceeded() {
		return errors.New("SSH output limit exceeded")
	}
	if err != nil {
		return fmt.Errorf("SSH command failed: %w", err)
	}

	return nil
}
