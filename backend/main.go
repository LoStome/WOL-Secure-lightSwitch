package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"secure-switch-backend/internal/auth"
	"secure-switch-backend/internal/config"
	"secure-switch-backend/internal/device"
	"secure-switch-backend/internal/server"
	"secure-switch-backend/internal/sshrunner"
	"secure-switch-backend/internal/store"
	"secure-switch-backend/internal/wol"
)

func main() {
	secret, err := auth.LoadJWTSecret()
	if err != nil {
		log.Fatalf("Invalid JWT configuration: %v", err)
	}

	fmt.Println("Main Starting...")

	addUserEmail := flag.String("adduser", "", "Email of the user to add")
	addUserPass := flag.String("password", "", "Password for the new user")
	addUserAdmin := flag.Bool("admin", false, "Make the new user an admin")
	addUserDevices := flag.String("devices", "", "Comma-separated list of allowed device IDs")
	flag.Parse()

	repository, err := store.Open("data/secure-switch.db")
	if err != nil {
		repository, err = store.Open("../data/secure-switch.db")
		if err != nil {
			log.Fatalf("failed to connect database: %v", err)
		}
	}
	if err := repository.Migrate(); err != nil {
		log.Fatalf("failed to migrate database: %v", err)
	}
	fmt.Println("Database initialized successfully.")

	if *addUserEmail != "" {
		if *addUserPass == "" {
			log.Fatal("Password is required when adding a user")
		}
		hash, err := auth.HashPassword(*addUserPass)
		if err != nil {
			log.Fatalf("Error hashing password: %v", err)
		}
		devices := []string{}
		if *addUserDevices != "" {
			devices = strings.Split(*addUserDevices, ",")
		}
		if err := repository.CreateUser(*addUserEmail, hash, *addUserAdmin, devices); err != nil {
			log.Fatalf("Error creating user: %v", err)
		}
		fmt.Printf("User %s created successfully.\n", *addUserEmail)
		os.Exit(0)
	}

	trustedProxies, err := auth.ParseTrustedProxies(os.Getenv("TRUSTED_PROXIES"))
	if err != nil {
		log.Fatalf("Invalid TRUSTED_PROXIES configuration: %v", err)
	}
	loader := &config.Loader{}
	monitor := device.NewMonitor()
	application := &server.App{
		Config:   loader,
		Store:    repository,
		Auth:     &auth.Service{Secret: secret, Users: repository},
		Limiter:  auth.NewDefaultLoginAttemptLimiter(),
		Monitor:  monitor,
		Wake:     wol.SendWol,
		Shutdown: sshrunner.RemoteShutdown,
	}
	router, err := application.Router(trustedProxies)
	if err != nil {
		log.Fatalf("Failed to configure HTTP router: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pingDone := make(chan struct{})
	go func() {
		defer close(pingDone)
		monitor.StartPingManager(ctx, loader.LoadHosts)
	}()

	port := os.Getenv("PORT")
	if port == "" {
		port = "7500"
	}

	bindAddress := strings.TrimSpace(os.Getenv("BIND_ADDRESS"))
	if bindAddress == "" {
		bindAddress = "127.0.0.1"
	}

	httpServer := server.NewHTTPServer(router, net.JoinHostPort(bindAddress, port))
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- httpServer.ListenAndServe() }()
	select {
	case err := <-serveErrors:
		stop()
		<-pingDone
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	case <-ctx.Done():
		stop()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := httpServer.Shutdown(shutdownCtx)
		cancel()
		if err != nil {
			httpServer.Close()
		}
		<-serveErrors
		<-pingDone
		if err != nil {
			log.Fatal(err)
		}
	}
}
