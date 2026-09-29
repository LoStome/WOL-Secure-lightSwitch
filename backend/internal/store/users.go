package store

import (
	"fmt"

	"gorm.io/gorm"
)

func (s *Store) GetUserByEmail(email string) (*User, error) {
	var user User
	result := s.DB.Preload("Devices").Where("email = ?", email).First(&user)
	if result.Error != nil {
		return nil, result.Error
	}
	return &user, nil
}

func (s *Store) GetUserByID(id uint) (*User, error) {
	var user User
	result := s.DB.Preload("Devices").First(&user, id)
	if result.Error != nil {
		return nil, result.Error
	}
	return &user, nil
}

func (s *Store) CreateUser(email string, passwordHash string, isAdmin bool, deviceIDs []string) error {
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

	return s.DB.Create(&user).Error
}

func (s *Store) UpdateUser(userID uint, passwordHash *string, isAdmin *bool, deviceIDs *[]string) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
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

		if deviceIDs == nil {
			return nil
		}
		if err := tx.Where("user_id = ?", userID).Delete(&UserDevice{}).Error; err != nil {
			return err
		}
		for _, devID := range *deviceIDs {
			if err := tx.Create(&UserDevice{UserID: userID, DeviceID: devID}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) RevokeUserTokens(userID, tokenVersion uint) error {
	result := s.DB.Model(&User{}).
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
