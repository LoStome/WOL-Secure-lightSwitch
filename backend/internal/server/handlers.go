package server

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"secure-switch-backend/internal/auth"
	"secure-switch-backend/internal/store"
)

// ----------------- API Handlers -----------------

type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (a *App) HandleLogin(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	email, err := ValidateEmail(req.Email)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := ValidatePassword(req.Password, false); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Email = email

	clientIP := c.ClientIP()
	if retryAfter := a.Limiter.RetryAfter(clientIP, req.Email, time.Now()); retryAfter > 0 {
		a.Limiter.RecordRateLimited()
		c.Header("Retry-After", strconv.Itoa(int(math.Ceil(retryAfter.Seconds()))))
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many login attempts; try again later"})
		return
	}

	user, err := a.Store.GetUserByEmail(req.Email)
	if err != nil {
		// If no administrator exists, atomically create this user as the first one.
		hasAdmins, dbErr := a.Store.HasAdmins()
		if dbErr == nil && !hasAdmins {
			if err := ValidatePassword(req.Password, true); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			hash, hashErr := auth.HashPassword(req.Password)
			if hashErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
				return
			}
			err = a.Store.CreateInitialAdmin(req.Email, hash)
			if err != nil {
				if errors.Is(err, store.ErrInitialAdminExists) {
					a.Limiter.RecordFailure(clientIP, req.Email, time.Now())
					c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
				} else {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create initial admin user"})
				}
				return
			}
			// Fetch the newly created user
			user, err = a.Store.GetUserByEmail(req.Email)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve new admin user"})
				return
			}
		} else {
			auth.CheckPasswordHash(req.Password, auth.DummyPasswordHash)
			a.Limiter.RecordFailure(clientIP, req.Email, time.Now())
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
			return
		}
	} else if !auth.CheckPasswordHash(req.Password, user.PasswordHash) {
		a.Limiter.RecordFailure(clientIP, req.Email, time.Now())
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}
	a.Limiter.RecordSuccess(clientIP, req.Email)

	token, err := a.Auth.GenerateJWT(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	auth.SetAuthCookie(c, token)
	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":       user.ID,
			"email":    user.Email,
			"is_admin": user.IsAdmin,
		},
	})
}

func (a *App) handleLoginMetrics(c *gin.Context) {
	c.JSON(http.StatusOK, a.Limiter.Metrics())
}

func (a *App) HandleLogout(c *gin.Context) {
	if err := a.Store.RevokeUserTokens(c.GetUint("userID"), c.GetUint("tokenVersion")); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
		return
	}

	auth.ClearAuthCookie(c)
	c.Status(http.StatusNoContent)
}

func (a *App) handleCurrentUser(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"id":       c.GetUint("userID"),
		"email":    c.GetString("userEmail"),
		"is_admin": c.GetBool("isAdmin"),
	})
}

// Check if user is authorized for a specific device based on UserDevice mapping
func (a *App) isAuthorizedForDevice(userID uint, deviceID string, isAdmin bool) bool {
	if isAdmin {
		return true
	}
	user, err := a.Store.GetUserByID(userID)
	if err != nil {
		return false
	}
	for _, dev := range user.Devices {
		if dev.DeviceID == deviceID {
			return true
		}
	}
	return false
}

func (a *App) handleGetHosts(c *gin.Context) {
	hosts, err := a.Config.LoadHosts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error loading hosts"})
		return
	}

	userID := c.GetUint("userID")
	isAdmin := c.GetBool("isAdmin")

	// Filter hosts based on authorization
	var authorizedHosts []Host
	for i := range hosts {
		if a.isAuthorizedForDevice(userID, hosts[i].ID, isAdmin) {
			// Attach online state to hosts from cache
			state := a.Monitor.State(hosts[i].ID)
			hosts[i].Online = state.Online
			if state.LastPinged == "" {
				hosts[i].LastPinged = "N/A"
			} else {
				hosts[i].LastPinged = state.LastPinged
			}

			authorizedHosts = append(authorizedHosts, hosts[i])
		}
	}

	c.JSON(http.StatusOK, authorizedHosts)
}

func (a *App) handleWOL(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetUint("userID")
	isAdmin := c.GetBool("isAdmin")

	if !a.isAuthorizedForDevice(userID, id, isAdmin) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized to access this device"})
		return
	}

	target, err := a.findHost(id)
	if err != nil {
		respondActionFailure(c, "wol target lookup", http.StatusNotFound, "Device not found")
		return
	}

	if err := a.Wake(target); err != nil {
		respondActionFailure(c, "wake-on-LAN", http.StatusInternalServerError, "Unable to send Wake-on-LAN packet")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Magic Packet sent successfully to " + target.Name})
}

func (a *App) handleShutdown(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetUint("userID")
	isAdmin := c.GetBool("isAdmin")

	if !a.isAuthorizedForDevice(userID, id, isAdmin) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized to access this device"})
		return
	}

	target, err := a.findHost(id)
	if err != nil {
		respondActionFailure(c, "shutdown target lookup", http.StatusNotFound, "Device not found")
		return
	}

	err = a.Shutdown(target)
	if err != nil {
		respondActionFailure(c, "shutdown", http.StatusInternalServerError, "Unable to shut down device")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Shutdown command received from " + target.Name})
}

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

func (a *App) handleCheckSetup(c *gin.Context) {
	hasAdmins, err := a.Store.HasAdmins()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check setup status"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"needs_setup": !hasAdmins})
}
