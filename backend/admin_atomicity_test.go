package main

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupConcurrentTestDB(t *testing.T) {
	t.Helper()

	previousDB := DB
	databasePath := filepath.Join(t.TempDir(), "secure-switch.db")
	var err error
	DB, err = gorm.Open(sqlite.Open(sqliteDSN(databasePath)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := DB.AutoMigrate(&User{}, &UserDevice{}); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	sqlDB, err := DB.DB()
	if err != nil {
		t.Fatalf("get database handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(16)
	t.Cleanup(func() {
		_ = sqlDB.Close()
		DB = previousDB
	})
}

func runConcurrently(count int, operation func(index int) error) []error {
	start := make(chan struct{})
	errorsByOperation := make([]error, count)
	var waitGroup sync.WaitGroup
	waitGroup.Add(count)
	for index := 0; index < count; index++ {
		go func() {
			defer waitGroup.Done()
			<-start
			errorsByOperation[index] = operation(index)
		}()
	}
	close(start)
	waitGroup.Wait()
	return errorsByOperation
}

func TestCreateInitialAdminIsAtomic(t *testing.T) {
	setupConcurrentTestDB(t)

	operationErrors := runConcurrently(12, func(index int) error {
		return CreateInitialAdmin(
			"admin"+string(rune('a'+index))+"@example.com",
			"unused-password-hash",
		)
	})

	successes := 0
	for _, err := range operationErrors {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrInitialAdminExists):
		default:
			t.Fatalf("CreateInitialAdmin returned unexpected error: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("successful initial admin creations = %d, want 1", successes)
	}

	adminCount, err := GetAdminCount()
	if err != nil {
		t.Fatalf("count administrators: %v", err)
	}
	if adminCount != 1 {
		t.Fatalf("administrator count = %d, want 1", adminCount)
	}
}

func TestConcurrentAdminDemotionPreservesOneAdmin(t *testing.T) {
	setupConcurrentTestDB(t)
	admins := createTestAdmins(t)
	demote := false

	operationErrors := runConcurrently(len(admins), func(index int) error {
		return UpdateUser(admins[index].ID, nil, &demote, nil)
	})
	assertOneAdminOperationSucceeds(t, operationErrors)
	assertAdminCount(t, 1)
}

func TestConcurrentAdminDeletionPreservesOneAdmin(t *testing.T) {
	setupConcurrentTestDB(t)
	admins := createTestAdmins(t)

	operationErrors := runConcurrently(len(admins), func(index int) error {
		return DeleteUser(admins[index].ID)
	})
	assertOneAdminOperationSucceeds(t, operationErrors)
	assertAdminCount(t, 1)
}

func createTestAdmins(t *testing.T) []User {
	t.Helper()
	admins := []User{
		{Email: "first-admin@example.com", PasswordHash: "unused", IsAdmin: true},
		{Email: "second-admin@example.com", PasswordHash: "unused", IsAdmin: true},
	}
	for index := range admins {
		if err := DB.Create(&admins[index]).Error; err != nil {
			t.Fatalf("create administrator: %v", err)
		}
	}
	return admins
}

func assertOneAdminOperationSucceeds(t *testing.T, operationErrors []error) {
	t.Helper()
	successes := 0
	lastAdminFailures := 0
	for _, err := range operationErrors {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrLastAdministrator):
			lastAdminFailures++
		default:
			t.Fatalf("administrator operation returned unexpected error: %v", err)
		}
	}
	if successes != 1 || lastAdminFailures != 1 {
		t.Fatalf("successes = %d, last-admin failures = %d; want 1 and 1", successes, lastAdminFailures)
	}
}

func assertAdminCount(t *testing.T, want int64) {
	t.Helper()
	adminCount, err := GetAdminCount()
	if err != nil {
		t.Fatalf("count administrators: %v", err)
	}
	if adminCount != want {
		t.Fatalf("administrator count = %d, want %d", adminCount, want)
	}
}
