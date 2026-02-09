package main

import (
	"os/exec"
	"runtime"
)

func IsOnline(ip string) bool {
	var cmd *exec.Cmd

	//different commands beetween windows and unix systems
	if runtime.GOOS == "windows" {
		// -n 1 (1 packet), -w 1000 (wait 1000ms)
		cmd = exec.Command("ping", "-n", "1", "-w", "1000", ip)
	} else {
		// -c 1 (1 packet), -W 1 (wait 1 second)
		cmd = exec.Command("ping", "-c", "1", "-W", "1", ip)
	}
	   
	//host online if err == nil, offline otherwise
	err := cmd.Run()
	return err == nil
}