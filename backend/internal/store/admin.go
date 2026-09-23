package store

import "gorm.io/gorm"

func (s *Store) HasAdmins() (bool, error) {
	var count int64
	err := s.DB.Model(&User{}).Where("is_admin = ?", true).Count(&count).Error
	return count > 0, err
}

// CreateInitialAdmin atomically creates the first administrator. Keeping the
// existence check in the INSERT prevents concurrent setup requests from both
// observing an empty administrator set and creating separate accounts.
func (s *Store) CreateInitialAdmin(email string, passwordHash string) error {
	result := s.DB.Exec(`
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

func (s *Store) GetAdminCount() (int64, error) {
	var count int64
	err := s.DB.Model(&User{}).Where("is_admin = ?", true).Count(&count).Error
	return count, err
}

// DeleteUser atomically enforces that an administrator may only be deleted
// while another administrator exists.
func (s *Store) DeleteUser(userID uint) error {
	result := s.DB.Where(`
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
	if err := s.DB.Model(&User{}).Where("id = ?", userID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return gorm.ErrRecordNotFound
	}
	return ErrLastAdministrator
}
