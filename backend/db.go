package main

import (
	"errors"
	"fmt"
	"log"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

var (
	ErrInitialAdminExists = errors.New("an administrator already exists")
	ErrLastAdministrator  = errors.New("cannot remove the last administrator")
)

func sqliteDSN(path string) string {
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

func InitDB() {
	var err error
	DB, err = gorm.Open(sqlite.Open(sqliteDSN("data/secure-switch.db")), &gorm.Config{})
	if err != nil {
		// try fallback path if running from another dir
		DB, err = gorm.Open(sqlite.Open(sqliteDSN("../data/secure-switch.db")), &gorm.Config{})
		if err != nil {
			log.Fatalf("failed to connect database: %v", err)
		}
	}

	// Migrate the schema
	err = DB.AutoMigrate(&User{}, &UserDevice{})
	if err != nil {
		log.Fatalf("failed to migrate database: %v", err)
	}

	fmt.Println("Database initialized successfully.")
}

func GetUserByEmail(email string) (*User, error) {
	var user User
	result := DB.Preload("Devices").Where("email = ?", email).First(&user)
	if result.Error != nil {
		return nil, result.Error
	}
	return &user, nil
}

func GetUserByID(id uint) (*User, error) {
	var user User
	result := DB.Preload("Devices").First(&user, id)
	if result.Error != nil {
		return nil, result.Error
	}
	return &user, nil
}

func HasAdmins() (bool, error) {
	var count int64
	err := DB.Model(&User{}).Where("is_admin = ?", true).Count(&count).Error
	return count > 0, err
}

func CreateUser(email string, passwordHash string, isAdmin bool, deviceIDs []string) error {
	var devices []UserDevice
	for _, devID := range deviceIDs {
		devices = append(devices, UserDevice{DeviceID: devID})
	}

	user := User{
		Email:        email,
		PasswordHash: passwordHash,
		IsAdmin:      isAdmin,
		Devices:      devices,
	}

	return DB.Create(&user).Error
}

// CreateInitialAdmin atomically creates the first administrator. Keeping the
// existence check in the INSERT prevents concurrent setup requests from both
// observing an empty administrator set and creating separate accounts.
func CreateInitialAdmin(email string, passwordHash string) error {
	result := DB.Exec(`
		INSERT INTO users (email, password_hash, is_admin, token_version)
		SELECT ?, ?, ?, ?
		WHERE NOT EXISTS (SELECT 1 FROM users WHERE is_admin = ?)
	`, email, passwordHash, true, 0, true)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrInitialAdminExists
	}
	return nil
}

func GetAdminCount() (int64, error) {
	var count int64
	err := DB.Model(&User{}).Where("is_admin = ?", true).Count(&count).Error
	return count, err
}

func UpdateUser(userID uint, passwordHash *string, isAdmin *bool, deviceIDs []string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		updates := map[string]interface{}{
			// A no-op write makes this the transaction's first statement and
			// acquires SQLite's write lock before any dependent reads.
			"id": gorm.Expr("id"),
		}
		if passwordHash != nil && *passwordHash != "" {
			updates["password_hash"] = *passwordHash
			updates["token_version"] = gorm.Expr("token_version + 1")
		}
		if isAdmin != nil {
			updates["is_admin"] = *isAdmin
			if _, passwordChanged := updates["password_hash"]; !passwordChanged {
				updates["token_version"] = gorm.Expr(
					"token_version + CASE WHEN is_admin <> ? THEN 1 ELSE 0 END",
					*isAdmin,
				)
			}
		}

		query := tx.Model(&User{}).Where("id = ?", userID)
		if isAdmin != nil && !*isAdmin {
			query = query.Where(`
				(
					is_admin = ? OR
					(SELECT COUNT(*) FROM users AS administrators
					 WHERE administrators.is_admin = ?) > 1
				)
			`, false, true)
		}

		result := query.UpdateColumns(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			var count int64
			if err := tx.Model(&User{}).Where("id = ?", userID).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return gorm.ErrRecordNotFound
			}
			return ErrLastAdministrator
		}

		if err := tx.Where("user_id = ?", userID).Delete(&UserDevice{}).Error; err != nil {
			return err
		}
		for _, devID := range deviceIDs {
			if err := tx.Create(&UserDevice{UserID: userID, DeviceID: devID}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteUser atomically enforces that an administrator may only be deleted
// while another administrator exists.
func DeleteUser(userID uint) error {
	result := DB.Where(`
		id = ? AND (
			is_admin = ? OR
			(SELECT COUNT(*) FROM users AS administrators
			 WHERE administrators.is_admin = ?) > 1
		)
	`, userID, false, true).Delete(&User{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 1 {
		return nil
	}

	var count int64
	if err := DB.Model(&User{}).Where("id = ?", userID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return gorm.ErrRecordNotFound
	}
	return ErrLastAdministrator
}

func RevokeUserTokens(userID, tokenVersion uint) error {
	result := DB.Model(&User{}).
		Where("id = ? AND token_version = ?", userID, tokenVersion).
		UpdateColumn("token_version", gorm.Expr("token_version + ?", 1))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("token is no longer valid")
	}
	return nil
}
