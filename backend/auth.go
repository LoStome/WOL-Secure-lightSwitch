package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var jwtSecret []byte

const minimumJWTSecretLength = 32

var insecureJWTSecrets = map[string]struct{}{
	"default-insecure-secret-change-me":    {},
	"your_very_long_and_random_jwt_secret": {},
}

func loadJWTSecret() ([]byte, error) {
	secretFile := strings.TrimSpace(os.Getenv("JWT_SECRET_FILE"))
	var secret string

	if secretFile != "" {
		contents, err := os.ReadFile(secretFile)
		if err != nil {
			return nil, fmt.Errorf("read JWT_SECRET_FILE: %w", err)
		}
		secret = strings.TrimSpace(string(contents))
	} else {
		secret = strings.TrimSpace(os.Getenv("JWT_SECRET"))
	}

	if secret == "" {
		return nil, errors.New("JWT secret is required: set JWT_SECRET_FILE or JWT_SECRET")
	}
	if utf8.RuneCountInString(secret) < minimumJWTSecretLength {
		return nil, fmt.Errorf("JWT secret must be at least %d characters", minimumJWTSecretLength)
	}

	placeholder := strings.Trim(secret, "\"'")
	if _, insecure := insecureJWTSecrets[placeholder]; insecure {
		return nil, errors.New("JWT secret must not use a known placeholder")
	}

	return []byte(secret), nil
}

func initializeJWTSecret() error {
	secret, err := loadJWTSecret()
	if err != nil {
		return err
	}
	jwtSecret = secret
	return nil
}

// HashPassword generates a bcrypt hash of the password
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

// CheckPasswordHash compares a securely hashed password to the plaintext one
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

type Claims struct {
	TokenVersion uint `json:"token_version"`
	jwt.RegisteredClaims
}

const jwtLifetime = time.Hour

// GenerateJWT creates a new JWT for an authenticated user
func GenerateJWT(user *User) (string, error) {
	now := time.Now()
	claims := &Claims{
		TokenVersion: user.TokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatUint(uint64(user.ID), 10),
			ExpiresAt: jwt.NewNumericDate(now.Add(jwtLifetime)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// ValidateJWT parses the JWT string and returns claims if valid
func ValidateJWT(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

// AuthMiddleware is a Gin middleware that ensures the request contains a valid JWT Bearer token
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header format must be Bearer {token}"})
			return
		}

		claims, err := ValidateJWT(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}
		userID, err := strconv.ParseUint(claims.Subject, 10, strconv.IntSize)
		if err != nil || userID == 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		user, err := GetUserByID(uint(userID))
		if err != nil || user.TokenVersion != claims.TokenVersion {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		// Attach current database values rather than trusting mutable JWT claims.
		c.Set("userID", user.ID)
		c.Set("userEmail", user.Email)
		c.Set("isAdmin", user.IsAdmin)
		c.Set("tokenVersion", user.TokenVersion)

		c.Next()
	}
}

// AdminMiddleware ensures the logged-in user is an admin
func AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		isAdmin, exists := c.Get("isAdmin")
		if !exists || isAdmin != true {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			return
		}
		c.Next()
	}
}
