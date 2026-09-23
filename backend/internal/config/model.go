package config

import (
	"errors"
	"fmt"
)

// Host is a device defined in hosts.yaml and returned by the hosts API.
type Host struct {
	ID             string   `yaml:"id"`
	Name           string   `yaml:"name"`
	MAC            string   `yaml:"mac"`
	IP             string   `yaml:"ip"`
	WolInterface   string   `yaml:"wol_interface" json:"-"`
	User           string   `yaml:"user" json:"-"`
	PasswordFile   string   `yaml:"password_file" json:"-"`
	KeyPath        string   `yaml:"key_path" json:"-"`
	Cmd            string   `yaml:"cmd" json:"-"`
	SkipInterfaces []string `yaml:"skip_interfaces" json:"-"`
	PingInterval   int      `yaml:"ping_interval" json:"ping_interval"`
	Online         bool     `yaml:"-" json:"online"`
	LastPinged     string   `yaml:"-" json:"last_pinged"`
}

const maxDeviceIDLength = 64

// ValidateDeviceID applies the same rules to configured hosts and user assignments.
func ValidateDeviceID(id string) error {
	if id == "" {
		return errors.New("device ID is required")
	}
	if len(id) > maxDeviceIDLength {
		return fmt.Errorf("device ID must be at most %d characters", maxDeviceIDLength)
	}
	for index := 0; index < len(id); index++ {
		character := id[index]
		isLetterOrDigit := character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9'
		if (index == 0 && !isLetterOrDigit) ||
			(index > 0 && !isLetterOrDigit && character != '.' && character != '_' && character != '-') {
			return errors.New("device ID must start with a letter or number and contain only letters, numbers, '.', '_' or '-'")
		}
	}
	return nil
}
