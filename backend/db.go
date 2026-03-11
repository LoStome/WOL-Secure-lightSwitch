package main

import (
	"fmt"
	"log"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

// User represents an administrator or user in the system
type User struct {
	ID           uint         `gorm:"primaryKey" json:"id"`
	Email        string       `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash string       `gorm:"not null" json:"-"`
	IsAdmin      bool         `gorm:"default:false" json:"is_admin"`
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
	DB, err = gorm.Open(sqlite.Open("data/secure-switch.db"), &gorm.Config{})
	if err != nil {
		// try fallback path if running from another dir
		DB, err = gorm.Open(sqlite.Open("../data/secure-switch.db"), &gorm.Config{})
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

func GetAdminCount() (int64, error) {
	var count int64
	err := DB.Model(&User{}).Where("is_admin = ?", true).Count(&count).Error
	return count, err
}

func UpdateUser(userID uint, passwordHash *string, isAdmin *bool, deviceIDs []string) error {
	var user User
	// Fetch the user
	if err := DB.First(&user, userID).Error; err != nil {
		return err
	}

	// Prevent demoting the last admin
	if user.IsAdmin && isAdmin != nil && !*isAdmin {
		count, err := GetAdminCount()
		if err != nil {
			return err
		}
		if count <= 1 {
			return fmt.Errorf("cannot demote the last administrator")
		}
	}

	// Update fields if provided
	if passwordHash != nil && *passwordHash != "" {
		user.PasswordHash = *passwordHash
	}
	if isAdmin != nil {
		user.IsAdmin = *isAdmin
	}

	// Begin transaction to ensure data integrity
	tx := DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	// Save the base user
	if err := tx.Save(&user).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Delete existing devices for the user
	if err := tx.Where("user_id = ?", userID).Delete(&UserDevice{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Insert new devices
	for _, devID := range deviceIDs {
		newDevice := UserDevice{
			UserID:   userID,
			DeviceID: devID,
		}
		if err := tx.Create(&newDevice).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	// Commit the transaction
	return tx.Commit().Error
}
