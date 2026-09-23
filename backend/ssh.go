package main

import "secure-switch-backend/internal/sshrunner"

func RemoteShutdown(h *Host) error { return sshrunner.RemoteShutdown(h) }
