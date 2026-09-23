package store

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupConcurrentTestDB(t *testing.T) *Store {
	t.Helper()

	databasePath := filepath.Join(t.TempDir(), "secure-switch.db")
	db, err := gorm.Open(sqlite.Open(SQLiteDSN(databasePath)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := db.AutoMigrate(&User{}, &UserDevice{}); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get database handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(16)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return &Store{DB: db}
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
	repository := setupConcurrentTestDB(t)

	operationErrors := runConcurrently(12, func(index int) error {
		return repository.CreateInitialAdmin(
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

	adminCount, err := repository.GetAdminCount()
	if err != nil {
		t.Fatalf("count administrators: %v", err)
	}
	if adminCount != 1 {
		t.Fatalf("administrator count = %d, want 1", adminCount)
	}
}

func TestConcurrentAdminDemotionPreservesOneAdmin(t *testing.T) {
	repository := setupConcurrentTestDB(t)
	admins := createTestAdmins(t, repository)
	demote := false

	operationErrors := runConcurrently(len(admins), func(index int) error {
		return repository.UpdateUser(admins[index].ID, nil, &demote, nil)
	})
	assertOneAdminOperationSucceeds(t, operationErrors)
	assertAdminCount(t, repository, 1)
}

func TestConcurrentAdminDeletionPreservesOneAdmin(t *testing.T) {
	repository := setupConcurrentTestDB(t)
	admins := createTestAdmins(t, repository)

	operationErrors := runConcurrently(len(admins), func(index int) error {
		return repository.DeleteUser(admins[index].ID)
	})
	assertOneAdminOperationSucceeds(t, operationErrors)
	assertAdminCount(t, repository, 1)
}

func createTestAdmins(t *testing.T, repository *Store) []User {
	t.Helper()
	admins := []User{
		{Email: "first-admin@example.com", PasswordHash: "unused", IsAdmin: true},
		{Email: "second-admin@example.com", PasswordHash: "unused", IsAdmin: true},
	}
	for index := range admins {
		if err := repository.DB.Create(&admins[index]).Error; err != nil {
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

func assertAdminCount(t *testing.T, repository *Store, want int64) {
	t.Helper()
	adminCount, err := repository.GetAdminCount()
	if err != nil {
		t.Fatalf("count administrators: %v", err)
	}
	if adminCount != want {
		t.Fatalf("administrator count = %d, want %d", adminCount, want)
	}
}
