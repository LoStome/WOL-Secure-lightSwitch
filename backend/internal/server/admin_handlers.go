package server

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"secure-switch-backend/internal/auth"
	"secure-switch-backend/internal/store"
)

// ---- Admin API ----

func (a *App) handleGetUsers(c *gin.Context) {
	var users []store.User
	// Preload the devices for the users so the admin can see them
	if err := a.Store.DB.Preload("Devices").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
		return
	}

	// We don't want to return password hashes, so we clear them manually or map to a DTO
	// However, json:"-" on PasswordHash already hides it.
	c.JSON(http.StatusOK, users)
}

type CreateUserRequest struct {
	Email    string   `json:"email" binding:"required"`
	Password string   `json:"password" binding:"required"`
	IsAdmin  bool     `json:"is_admin"`
	Devices  []string `json:"devices"`
}

func (a *App) handleCreateUser(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	email, err := ValidateEmail(req.Email)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := ValidatePassword(req.Password, true); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.Devices) > 0 {
		configured, err := a.configuredDeviceIDs()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load configured devices"})
			return
		}
		if err := ValidateDeviceIDs(req.Devices, configured); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	err = a.Store.CreateUser(email, hash, req.IsAdmin, req.Devices)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "User created successfully"})
}

type UpdateUserRequest struct {
	Password *string   `json:"password"` // optional
	IsAdmin  *bool     `json:"is_admin"` // optional
	Devices  *[]string `json:"devices"`  // optional; an empty list clears assignments
}

func (a *App) HandleUpdateUser(c *gin.Context) {
	id := c.Param("id")
	userID, err := strconv.Atoi(id)
	if err != nil || userID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Password != nil {
		if err := ValidatePassword(*req.Password, true); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	if req.Devices != nil && len(*req.Devices) > 0 {
		configured, err := a.configuredDeviceIDs()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load configured devices"})
			return
		}
		if err := ValidateDeviceIDs(*req.Devices, configured); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	var hashPtr *string
	if req.Password != nil {
		hash, err := auth.HashPassword(*req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}
		hashPtr = &hash
	}

	err = a.Store.UpdateUser(uint(userID), hashPtr, req.IsAdmin, req.Devices)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User updated successfully"})
}

func (a *App) handleDeleteUser(c *gin.Context) {
	id := c.Param("id")
	userID, err := strconv.Atoi(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	if err := a.Store.DeleteUser(uint(userID)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		if errors.Is(err, store.ErrLastAdministrator) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete the last administrator"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}
