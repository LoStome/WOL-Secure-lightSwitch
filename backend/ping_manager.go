package main

import (
	"context"

	"secure-switch-backend/internal/device"
)

// The main package shares one monitor until the server package is extracted.
var hostStates = device.NewMonitor()

func StartPingManager(ctx context.Context) {
	hostStates.StartPingManager(ctx, LoadHosts)
}
