package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

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
		Timeout:         5 * time.Second,
	}

	//Connection to the SSH server
	client, err := ssh.Dial("tcp", address, config)
	if err != nil {
		return errors.New("SSH connection failed")
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return errors.New("SSH session failed")
	}
	defer session.Close()

	// Execute the shutdown command without logging command arguments or remote output.
	if _, err := session.CombinedOutput(command); err != nil {
		return fmt.Errorf("SSH command failed: %w", err)
	}

	return nil
}
