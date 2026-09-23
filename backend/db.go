package main

import (
	"fmt"
	"log"

	"gorm.io/gorm"
	"secure-switch-backend/internal/store"
)

// Temporary compatibility adapters while handlers move into internal/server.
var DB *gorm.DB

type User = store.User
type UserDevice = store.UserDevice

var ErrInitialAdminExists = store.ErrInitialAdminExists
var ErrLastAdministrator = store.ErrLastAdministrator

func sqliteDSN(path string) string { return store.SQLiteDSN(path) }

func InitDB() {
	repository, err := store.Open("data/secure-switch.db")
	if err != nil {
		repository, err = store.Open("../data/secure-switch.db")
		if err != nil {
			log.Fatalf("failed to connect database: %v", err)
		}
	}
	DB = repository.DB
	if err := repository.Migrate(); err != nil {
		log.Fatalf("failed to migrate database: %v", err)
	}
	fmt.Println("Database initialized successfully.")
}

func currentStore() *store.Store { return &store.Store{DB: DB} }

func GetUserByEmail(email string) (*User, error) { return currentStore().GetUserByEmail(email) }
func GetUserByID(id uint) (*User, error)         { return currentStore().GetUserByID(id) }
func HasAdmins() (bool, error)                   { return currentStore().HasAdmins() }
func CreateUser(email, passwordHash string, isAdmin bool, deviceIDs []string) error {
	return currentStore().CreateUser(email, passwordHash, isAdmin, deviceIDs)
}
func CreateInitialAdmin(email, passwordHash string) error {
	return currentStore().CreateInitialAdmin(email, passwordHash)
}
func GetAdminCount() (int64, error) { return currentStore().GetAdminCount() }
func UpdateUser(userID uint, passwordHash *string, isAdmin *bool, deviceIDs *[]string) error {
	return currentStore().UpdateUser(userID, passwordHash, isAdmin, deviceIDs)
}
func DeleteUser(userID uint) error { return currentStore().DeleteUser(userID) }
func RevokeUserTokens(userID, tokenVersion uint) error {
	return currentStore().RevokeUserTokens(userID, tokenVersion)
}
