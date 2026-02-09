package main

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
)

func RemoteShutdown(h *Host) error {
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

	config := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	//Connection to the SSH server
	client, err := ssh.Dial("tcp", ip+":22", config)
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