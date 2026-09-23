package main

import "secure-switch-backend/internal/config"

// The main package keeps its existing call sites until the server package is extracted.
var hostLoader = &config.Loader{}

func LoadHosts() ([]Host, error) {
	return hostLoader.LoadHosts()
}

func loadHostsWithStatus() ([]Host, error) {
	return hostLoader.LoadHostsWithStatus()
}

func parseHosts(data []byte) ([]Host, error) {
	return config.ParseHosts(data)
}
