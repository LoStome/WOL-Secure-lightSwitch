package main

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func RemoteShutdown(h *Host) error {
	return remoteShutdown(h, h.IP+":22")
}

func remoteShutdown(h *Host, address string) error {
	var authMethods []ssh.AuthMethod

	ip := h.IP
	user := h.User
	password := h.Password
	keyPath := h.KeyPath
	command := h.Cmd

	//default shutdown command if not provided
	if command == "" {
		command = "sudo -n /usr/sbin/poweroff"
	}

	if keyPath != "" {

		key, err := os.ReadFile(keyPath)
		if err != nil {
			return fmt.Errorf("impossibile leggere la chiave: %v", err)
		}

		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return fmt.Errorf("impossibile decifrare la chiave: %v", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
		fmt.Println("Debug: Utilizzo autenticazione tramite Chiave SSH")

	} else if password != "" {

		authMethods = append(authMethods, ssh.Password(password))
		fmt.Println("Debug: Utilizzo autenticazione tramite Password")
	} else {
		return fmt.Errorf("nessun metodo di autenticazione fornito")
	}

	// Provision trusted host keys out of band; never accept new keys automatically.
	// Override the path with SSH_KNOWN_HOSTS_FILE for deployments outside Docker.
	knownHostsPath := os.Getenv("SSH_KNOWN_HOSTS_FILE")
	if knownHostsPath == "" {
		knownHostsPath = "data/.ssh/known_hosts"
	}
	hostKeyCallback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return fmt.Errorf("impossibile caricare known_hosts SSH: %w", err)
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
		return fmt.Errorf("connection failed: %v", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("session failed: %v", err)
	}
	defer session.Close()

	//Execute the shutdown command
	fmt.Printf("Eseguendo comando: %s su %s\n", command, ip)
	output, err := session.CombinedOutput(command)
	if err != nil {
		fmt.Printf("Errore catturato: %v\n", err)
	}
	fmt.Printf("Output del server: %s\n", string(output))

	return nil
}
