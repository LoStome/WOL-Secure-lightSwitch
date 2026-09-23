package main

import (
	"context"
	"os/exec"
	"runtime"
	"time"
)

func IsOnline(ip string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return isOnlineContext(ctx, ip)
}

func isOnlineContext(ctx context.Context, ip string) bool {
	var cmd *exec.Cmd

	// Different commands between Windows and Unix systems.
	if runtime.GOOS == "windows" {
		// -n 1 (1 packet), -w 1000 (wait 1000ms)
		cmd = exec.CommandContext(ctx, "ping", "-n", "1", "-w", "1000", ip)
	} else {
		// -c 1 (1 packet), -W 1 (wait 1 second)
		cmd = exec.CommandContext(ctx, "ping", "-c", "1", "-W", "1", ip)
	}

	// Host is online if the command completed successfully.
	err := cmd.Run()
	return err == nil
}
