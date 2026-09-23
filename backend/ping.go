package main

import "secure-switch-backend/internal/device"

func IsOnline(ip string) bool { return device.IsOnline(ip) }
