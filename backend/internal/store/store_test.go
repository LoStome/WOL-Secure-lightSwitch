package store

import (
	"path/filepath"
	"testing"
)

func TestStoresUseIndependentDatabases(t *testing.T) {
	first, err := Open(filepath.Join(t.TempDir(), "first.db"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := Open(filepath.Join(t.TempDir(), "second.db"))
	if err != nil {
		t.Fatal(err)
	}
	for _, repository := range []*Store{first, second} {
		if err := repository.Migrate(); err != nil {
			t.Fatal(err)
		}
		sqlDB, err := repository.DB.DB()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	if err := first.CreateUser("admin@example.com", "hash", true, []string{"host-one"}); err != nil {
		t.Fatal(err)
	}
	if hasAdmins, err := first.HasAdmins(); err != nil || !hasAdmins {
		t.Fatalf("first store has admins = %t, err = %v", hasAdmins, err)
	}
	if hasAdmins, err := second.HasAdmins(); err != nil || hasAdmins {
		t.Fatalf("second store has admins = %t, err = %v", hasAdmins, err)
	}
	user, err := first.GetUserByEmail("admin@example.com")
	if err != nil || len(user.Devices) != 1 || user.Devices[0].DeviceID != "host-one" {
		t.Fatalf("first store user = %+v, err = %v", user, err)
	}
}
