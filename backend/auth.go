package main

import (
	"secure-switch-backend/internal/auth"

	"github.com/gin-gonic/gin"
)

// Temporary compatibility adapters while handlers move into internal/server.
var jwtSecret []byte

const minimumJWTSecretLength = auth.MinimumJWTSecretLength

type Claims = auth.Claims

func loadJWTSecret() ([]byte, error) { return auth.LoadJWTSecret() }
func initializeJWTSecret() error {
	secret, err := auth.LoadJWTSecret()
	if err != nil {
		return err
	}
	jwtSecret = secret
	return nil
}
func HashPassword(password string) (string, error) { return auth.HashPassword(password) }
func CheckPasswordHash(password, hash string) bool { return auth.CheckPasswordHash(password, hash) }
func setAuthCookie(c *gin.Context, token string)   { auth.SetAuthCookie(c, token) }
func clearAuthCookie(c *gin.Context)               { auth.ClearAuthCookie(c) }

func authService() *auth.Service {
	return &auth.Service{Secret: jwtSecret, Users: currentStore()}
}
func GenerateJWT(user *User) (string, error)          { return authService().GenerateJWT(user) }
func ValidateJWT(tokenString string) (*Claims, error) { return authService().ValidateJWT(tokenString) }
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) { authService().AuthMiddleware()(c) }
}
func AdminMiddleware() gin.HandlerFunc { return auth.AdminMiddleware() }
