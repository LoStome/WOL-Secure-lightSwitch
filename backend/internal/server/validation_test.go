package server

import (
	"strings"
	"testing"
)

func TestValidationRules(t *testing.T) {
	t.Run("email", func(t *testing.T) {
		if got, err := ValidateEmail(" user@example.com "); err != nil || got != "user@example.com" {
			t.Fatalf("ValidateEmail() = %q, %v; want trimmed valid email", got, err)
		}
		for _, email := range []string{"not-an-email", "Display Name <user@example.com>", strings.Repeat("a", MaxEmailBytes+1)} {
			if _, err := ValidateEmail(email); err == nil {
				t.Errorf("ValidateEmail(%q) succeeded, want an error", email)
			}
		}
	})

	t.Run("password", func(t *testing.T) {
		if err := ValidatePassword(strings.Repeat("a", MinimumPasswordLength), true); err != nil {
			t.Fatalf("minimum password rejected: %v", err)
		}
		for _, password := range []string{"short", strings.Repeat("a", MaxPasswordBytes+1)} {
			if err := ValidatePassword(password, true); err == nil {
				t.Errorf("ValidatePassword(%q) succeeded, want an error", password)
			}
		}
	})

	t.Run("device IDs", func(t *testing.T) {
		configured := map[string]struct{}{"server-proxmox": {}}
		if err := ValidateDeviceIDs([]string{"server-proxmox"}, configured); err != nil {
			t.Fatalf("configured device ID rejected: %v", err)
		}
		for _, deviceIDs := range [][]string{
			{"unknown-device"},
			{"server-proxmox", "server-proxmox"},
			{"invalid/device"},
		} {
			if err := ValidateDeviceIDs(deviceIDs, configured); err == nil {
				t.Errorf("ValidateDeviceIDs(%v) succeeded, want an error", deviceIDs)
			}
		}
	})
}
