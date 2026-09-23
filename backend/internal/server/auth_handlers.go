package server

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"secure-switch-backend/internal/auth"
	"secure-switch-backend/internal/store"
)

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

func (a *App) handleCheckSetup(c *gin.Context) {
	hasAdmins, err := a.Store.HasAdmins()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check setup status"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"needs_setup": !hasAdmins})
}
