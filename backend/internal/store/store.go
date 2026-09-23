package store

import (
	"errors"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type Store struct{ DB *gorm.DB }

var (
	ErrInitialAdminExists = errors.New("an administrator already exists")
	ErrLastAdministrator  = errors.New("cannot remove the last administrator")
)

func SQLiteDSN(path string) string {
	return path + "?_pragma=busy_timeout%3d10000"
}

// User represents an administrator or user in the system
type User struct {
	ID           uint         `gorm:"primaryKey" json:"id"`
	Email        string       `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash string       `gorm:"not null" json:"-"`
	IsAdmin      bool         `gorm:"default:false" json:"is_admin"`
	TokenVersion uint         `gorm:"not null;default:0" json:"-"`
	Devices      []UserDevice `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE;" json:"devices"`
}

// UserDevice represents the devices a specific user is authorized to manage
type UserDevice struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	UserID   uint   `gorm:"index;not null" json:"user_id"`
	DeviceID string `gorm:"index;not null" json:"device_id"` // Matches the ID in hosts.yaml
}

func Open(path string) (*Store, error) {
	db, err := gorm.Open(sqlite.Open(SQLiteDSN(path)), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return &Store{DB: db}, nil
}

func (s *Store) Migrate() error {
	return s.DB.AutoMigrate(&User{}, &UserDevice{})
}
