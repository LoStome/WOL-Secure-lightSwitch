package config

import (
	"errors"
	"os"
)

// EnsureInitialHosts creates an empty device list on the first container start.
// Existing configuration, including invalid configuration, is never replaced.
func EnsureInitialHosts(path string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := file.WriteString("[]\n"); err != nil {
		file.Close()
		os.Remove(path)
		return err
	}
	if err := file.Close(); err != nil {
		os.Remove(path)
		return err
	}
	return nil
}
