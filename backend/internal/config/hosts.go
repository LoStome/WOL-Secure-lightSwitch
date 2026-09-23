package config

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

type Loader struct {
	sync.Mutex
	workingDir  string
	path        string
	modTime     time.Time
	size        int64
	checked     bool
	hosts       []Host
	loaded      bool
	reloadError error
}

// LoadHosts returns the latest complete configuration, including when a reload fails.
func (l *Loader) LoadHosts() ([]Host, error) {
	hosts, err := l.LoadHostsWithStatus()
	if hosts != nil {
		return hosts, nil
	}
	return nil, err
}

// LoadHostsWithStatus also reports errors in the current file for /healthz.
func (l *Loader) LoadHostsWithStatus() ([]Host, error) {
	l.Lock()
	defer l.Unlock()

	workingDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if workingDir != l.workingDir {
		l.workingDir = workingDir
		l.path = ""
		l.checked = false
		l.hosts = nil
		l.loaded = false
		l.reloadError = nil
	}

	path := filepath.Join(workingDir, "data", "hosts.yaml")
	info, err := os.Stat(path)
	if err != nil {
		path = filepath.Join(workingDir, "..", "data", "hosts.yaml")
		info, err = os.Stat(path)
	}
	if err != nil {
		l.reloadError = fmt.Errorf("stat hosts.yaml: %w", err)
		l.checked = false
		return copyHosts(l.hosts, l.loaded), l.reloadError
	}
	if !info.Mode().IsRegular() {
		l.reloadError = errors.New("hosts.yaml is not a regular file")
		l.checked = false
		return copyHosts(l.hosts, l.loaded), l.reloadError
	}
	if l.checked && l.path == path &&
		l.modTime.Equal(info.ModTime()) && l.size == info.Size() {
		return copyHosts(l.hosts, l.loaded), l.reloadError
	}

	l.path = path
	l.modTime = info.ModTime()
	l.size = info.Size()
	l.checked = true
	data, err := os.ReadFile(path)
	if err == nil {
		var parsed []Host
		parsed, err = ParseHosts(data)
		if err == nil {
			l.hosts = parsed
			l.loaded = true
		}
	} else {
		l.checked = false
	}
	l.reloadError = err
	return copyHosts(l.hosts, l.loaded), err
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

func ParseHosts(data []byte) ([]Host, error) {
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
		if err := ValidateDeviceID(host.ID); err != nil {
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
