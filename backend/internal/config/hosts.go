package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"

	"gopkg.in/yaml.v3"
)

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
