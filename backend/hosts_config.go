package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

var hostsSnapshot = struct {
	sync.Mutex
	workingDir  string
	path        string
	modTime     time.Time
	size        int64
	checked     bool
	hosts       []Host
	loaded      bool
	reloadError error
}{}

// LoadHosts returns the latest complete configuration, including when a reload fails.
func LoadHosts() ([]Host, error) {
	hosts, err := loadHostsWithStatus()
	if hosts != nil {
		return hosts, nil
	}
	return nil, err
}

// loadHostsWithStatus also reports errors in the current file for /healthz.
func loadHostsWithStatus() ([]Host, error) {
	hostsSnapshot.Lock()
	defer hostsSnapshot.Unlock()

	workingDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if workingDir != hostsSnapshot.workingDir {
		hostsSnapshot.workingDir = workingDir
		hostsSnapshot.path = ""
		hostsSnapshot.checked = false
		hostsSnapshot.hosts = nil
		hostsSnapshot.loaded = false
		hostsSnapshot.reloadError = nil
	}

	path := filepath.Join(workingDir, "data", "hosts.yaml")
	info, err := os.Stat(path)
	if err != nil {
		path = filepath.Join(workingDir, "..", "data", "hosts.yaml")
		info, err = os.Stat(path)
	}
	if err != nil {
		hostsSnapshot.reloadError = fmt.Errorf("stat hosts.yaml: %w", err)
		hostsSnapshot.checked = false
		return copyHosts(hostsSnapshot.hosts, hostsSnapshot.loaded), hostsSnapshot.reloadError
	}
	if !info.Mode().IsRegular() {
		hostsSnapshot.reloadError = errors.New("hosts.yaml is not a regular file")
		hostsSnapshot.checked = false
		return copyHosts(hostsSnapshot.hosts, hostsSnapshot.loaded), hostsSnapshot.reloadError
	}
	if hostsSnapshot.checked && hostsSnapshot.path == path &&
		hostsSnapshot.modTime.Equal(info.ModTime()) && hostsSnapshot.size == info.Size() {
		return copyHosts(hostsSnapshot.hosts, hostsSnapshot.loaded), hostsSnapshot.reloadError
	}

	hostsSnapshot.path = path
	hostsSnapshot.modTime = info.ModTime()
	hostsSnapshot.size = info.Size()
	hostsSnapshot.checked = true
	data, err := os.ReadFile(path)
	if err == nil {
		var parsed []Host
		parsed, err = parseHosts(data)
		if err == nil {
			hostsSnapshot.hosts = parsed
			hostsSnapshot.loaded = true
		}
	} else {
		hostsSnapshot.checked = false
	}
	hostsSnapshot.reloadError = err
	return copyHosts(hostsSnapshot.hosts, hostsSnapshot.loaded), err
}

func copyHosts(hosts []Host, loaded bool) []Host {
	if !loaded {
		return nil
	}
	copyOfHosts := make([]Host, len(hosts))
	copy(copyOfHosts, hosts)
	for i := range copyOfHosts {
		copyOfHosts[i].SkipInterfaces = append([]string(nil), hosts[i].SkipInterfaces...)
	}
	return copyOfHosts
}

func parseHosts(data []byte) ([]Host, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, errors.New("hosts.yaml is empty")
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var hosts []Host
	if err := decoder.Decode(&hosts); err != nil {
		return nil, err
	}
	if hosts == nil {
		return nil, errors.New("hosts.yaml must be a list of hosts")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("hosts.yaml contains multiple YAML documents")
		}
		return nil, err
	}
	seenIDs := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		if err := validateDeviceID(host.ID); err != nil {
			return nil, fmt.Errorf("invalid host ID %q: %w", host.ID, err)
		}
		if _, duplicate := seenIDs[host.ID]; duplicate {
			return nil, fmt.Errorf("duplicate host ID %q", host.ID)
		}
		seenIDs[host.ID] = struct{}{}
		if strings.TrimSpace(host.Name) == "" {
			return nil, fmt.Errorf("host %q: name is required", host.ID)
		}
		mac, err := net.ParseMAC(host.MAC)
		if err != nil || len(mac) != 6 {
			return nil, fmt.Errorf("host %q: invalid six-byte MAC address", host.ID)
		}
		if host.IP != "" && !validHostAddress(host.IP) {
			return nil, fmt.Errorf("host %q: invalid IP or hostname", host.ID)
		}
		if host.PingInterval < 0 {
			return nil, fmt.Errorf("host %q: ping_interval must not be negative", host.ID)
		}
		if host.KeyPath != "" && host.PasswordFile != "" {
			return nil, fmt.Errorf("host %q: configure only one SSH credential source", host.ID)
		}
	}
	return hosts, nil
}

func validHostAddress(address string) bool {
	if net.ParseIP(address) != nil {
		return true
	}
	numericAddress := true
	for _, char := range address {
		if (char < '0' || char > '9') && char != '.' {
			numericAddress = false
			break
		}
	}
	if numericAddress {
		return false
	}
	if len(address) > 253 || strings.HasSuffix(address, ".") {
		return false
	}
	for _, label := range strings.Split(address, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, char := range label {
			isLetter := char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z'
			isDigit := char >= '0' && char <= '9'
			if !isLetter && !isDigit && char != '-' {
				return false
			}
		}
	}
	return true
}
