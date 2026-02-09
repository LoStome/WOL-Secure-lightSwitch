package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)


func main() {
	fmt.Println("Wol Starting...")

	// Load .env file if it exists, but don't fail if it doesn't
	// This allows users to set environment variables directly in their system if they prefer, without needing a .env file.
    err := godotenv.Load()
    if err != nil {
        log.Println("Info: no .env file found, relying on system environment variables")
    }

    mac := os.Getenv("TARGET_MAC")
    if mac == "" {
        log.Fatal("ERROR: TARGET_MAC environment variable is not set!")
    }
	

	err  = SendWol(mac)
	if err != nil {
		fmt.Printf("Errore durante l'invio: %v\n", err)
	} else {
		fmt.Println("Test completato con successo!")
	}

	err = RemoteShutdown(
	)
	if err != nil {
		fmt.Printf("Errore durante lo spegnimento remoto: %v\n", err)
	} else {
		fmt.Println("Comando inviato con successo!")
	}

}