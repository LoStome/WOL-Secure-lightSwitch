package server

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"unicode/utf8"

	"secure-switch-backend/internal/config"
)

type Host = config.Host

const (
	MaxRequestBodyBytes   int64 = 64 * 1024
	MaxEmailBytes               = 254
	MinimumPasswordLength       = 12
	MaxPasswordBytes            = 72
)

func ValidateEmail(email string) (string, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return "", errors.New("email is required")
	}
	if len(email) > MaxEmailBytes {
		return "", fmt.Errorf("email must be at most %d bytes", MaxEmailBytes)
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email {
		return "", errors.New("email must be a valid address")
	}
	return email, nil
}

func ValidatePassword(password string, requireMinimum bool) error {
	if password == "" {
		return errors.New("password is required")
	}
	if !utf8.ValidString(password) {
		return errors.New("password must be valid UTF-8")
	}
	if requireMinimum && utf8.RuneCountInString(password) < MinimumPasswordLength {
		return fmt.Errorf("password must be at least %d characters", MinimumPasswordLength)
	}
	if len(password) > MaxPasswordBytes {
		return fmt.Errorf("password must be at most %d bytes", MaxPasswordBytes)
	}
	return nil
}

func validateDeviceID(id string) error {
	return config.ValidateDeviceID(id)
}

func (a *App) configuredDeviceIDs() (map[string]struct{}, error) {
	hosts, err := a.Config.LoadHosts()
	if err != nil {
		return nil, err
	}
	configured := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		configured[host.ID] = struct{}{}
	}
	return configured, nil
}

func ValidateDeviceIDs(deviceIDs []string, configured map[string]struct{}) error {
	seen := make(map[string]struct{}, len(deviceIDs))
	for _, deviceID := range deviceIDs {
		if err := validateDeviceID(deviceID); err != nil {
			return err
		}
		if _, duplicate := seen[deviceID]; duplicate {
			return fmt.Errorf("device ID %q must not be repeated", deviceID)
		}
		if _, exists := configured[deviceID]; !exists {
			return fmt.Errorf("device ID %q is not configured", deviceID)
		}
		seen[deviceID] = struct{}{}
	}
	return nil
}

func (a *App) findHost(id string) (*Host, error) {
	hosts, err := a.Config.LoadHosts()
	if err != nil {
		return nil, err
	}
	for i := range hosts {
		if hosts[i].ID == id {
			return &hosts[i], nil
		}
	}
	return nil, fmt.Errorf("host %s not found", id)
}
