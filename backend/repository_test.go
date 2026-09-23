package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRepositoryDoesNotContainDatabaseBackup(t *testing.T) {
	backupPath := filepath.Join("..", "data", "secure-switch.db.bak")

	_, err := os.Stat(backupPath)
	if err == nil {
		t.Fatalf("sensitive database backup must not be present in the repository: %s", backupPath)
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("check database backup: %v", err)
	}
}
