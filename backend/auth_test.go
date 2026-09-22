package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func TestLoginRateLimitsRepeatedInvalidPasswords(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupAuthTestDB(t)

	passwordHash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash test password: %v", err)
	}
	if err := DB.Create(&User{
		Email:        "user@example.com",
		PasswordHash: string(passwordHash),
		IsAdmin:      true,
	}).Error; err != nil {
		t.Fatalf("create test user: %v", err)
	}

	previousLimiter := loginLimiter
	loginLimiter = newLoginAttemptLimiter(2, time.Minute, time.Minute, loginMaxTrackedKeys, loginCleanupInterval)
	t.Cleanup(func() { loginLimiter = previousLimiter })

	router := gin.New()
	router.POST("/login", handleLogin)
	body := []byte(`{"email":"user@example.com","password":"wrong-password"}`)

	for attempt := 1; attempt <= 2; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("invalid login %d returned %d, want %d", attempt, response.Code, http.StatusUnauthorized)
		}
	}

	request := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("login after repeated failures returned %d, want %d", response.Code, http.StatusTooManyRequests)
	}
	if response.Header().Get("Retry-After") == "" {
		t.Fatal("rate-limited login did not include Retry-After")
	}
}

func TestLoadJWTSecretRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		secret string
	}{
		{name: "missing"},
		{name: "too short", secret: "short-secret"},
		{name: "old fallback", secret: "default-insecure-secret-change-me"},
		{name: "compose placeholder", secret: "your_very_long_and_random_jwt_secret"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("JWT_SECRET_FILE", "")
			t.Setenv("JWT_SECRET", test.secret)

			if _, err := loadJWTSecret(); err == nil {
				t.Fatal("loadJWTSecret() succeeded with an invalid secret")
			}
		})
	}
}

func TestLoadJWTSecretFromEnvironment(t *testing.T) {
	want := strings.Repeat("a", minimumJWTSecretLength)
	t.Setenv("JWT_SECRET_FILE", "")
	t.Setenv("JWT_SECRET", "  "+want+"  ")

	got, err := loadJWTSecret()
	if err != nil {
		t.Fatalf("loadJWTSecret() returned an error: %v", err)
	}
	if string(got) != want {
		t.Fatalf("loadJWTSecret() = %q, want %q", got, want)
	}
}

func TestLoadJWTSecretFromFile(t *testing.T) {
	want := strings.Repeat("b", minimumJWTSecretLength)
	secretFile := filepath.Join(t.TempDir(), "jwt_secret")
	if err := os.WriteFile(secretFile, []byte(want+"\n"), 0o600); err != nil {
		t.Fatalf("write test secret: %v", err)
	}
	t.Setenv("JWT_SECRET_FILE", secretFile)
	t.Setenv("JWT_SECRET", "")

	got, err := loadJWTSecret()
	if err != nil {
		t.Fatalf("loadJWTSecret() returned an error: %v", err)
	}
	if string(got) != want {
		t.Fatalf("loadJWTSecret() = %q, want %q", got, want)
	}
}

func TestLoadJWTSecretFileTakesPrecedence(t *testing.T) {
	want := strings.Repeat("c", minimumJWTSecretLength)
	secretFile := filepath.Join(t.TempDir(), "jwt_secret")
	if err := os.WriteFile(secretFile, []byte(want), 0o600); err != nil {
		t.Fatalf("write test secret: %v", err)
	}
	t.Setenv("JWT_SECRET_FILE", secretFile)
	t.Setenv("JWT_SECRET", "short-secret")

	got, err := loadJWTSecret()
	if err != nil {
		t.Fatalf("loadJWTSecret() returned an error: %v", err)
	}
	if string(got) != want {
		t.Fatalf("loadJWTSecret() = %q, want %q", got, want)
	}
}

func TestLoadJWTSecretDoesNotFallBackWhenFileIsMissing(t *testing.T) {
	t.Setenv("JWT_SECRET_FILE", filepath.Join(t.TempDir(), "missing"))
	t.Setenv("JWT_SECRET", strings.Repeat("d", minimumJWTSecretLength))

	if _, err := loadJWTSecret(); err == nil {
		t.Fatal("loadJWTSecret() succeeded with a missing configured secret file")
	}
}

func setupAuthTestDB(t *testing.T) {
	t.Helper()

	previousDB := DB
	var err error
	DB, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := DB.AutoMigrate(&User{}, &UserDevice{}); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	t.Cleanup(func() { DB = previousDB })
}

func authenticatedRequest(t *testing.T, router http.Handler, method, path, token string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(method, path, nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestAdminMiddlewareUsesCurrentDatabaseRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupAuthTestDB(t)
	jwtSecret = []byte(strings.Repeat("s", minimumJWTSecretLength))

	admin := User{Email: "admin@example.com", PasswordHash: "unused", IsAdmin: true}
	if err := DB.Create(&admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}
	token, err := GenerateJWT(&admin)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	router := gin.New()
	router.GET("/admin", AuthMiddleware(), AdminMiddleware(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	if response := authenticatedRequest(t, router, http.MethodGet, "/admin", token); response.Code != http.StatusNoContent {
		t.Fatalf("admin request before demotion returned %d, want %d", response.Code, http.StatusNoContent)
	}

	if err := DB.Model(&admin).Update("is_admin", false).Error; err != nil {
		t.Fatalf("demote admin: %v", err)
	}
	if response := authenticatedRequest(t, router, http.MethodGet, "/admin", token); response.Code != http.StatusForbidden {
		t.Fatalf("admin request after demotion returned %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestLogoutRevokesCurrentToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupAuthTestDB(t)
	jwtSecret = []byte(strings.Repeat("s", minimumJWTSecretLength))

	user := User{Email: "user@example.com", PasswordHash: "unused"}
	if err := DB.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	token, err := GenerateJWT(&user)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	router := gin.New()
	router.POST("/logout", AuthMiddleware(), handleLogout)
	router.GET("/protected", AuthMiddleware(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	if response := authenticatedRequest(t, router, http.MethodPost, "/logout", token); response.Code != http.StatusNoContent {
		t.Fatalf("logout returned %d, want %d", response.Code, http.StatusNoContent)
	}
	if response := authenticatedRequest(t, router, http.MethodGet, "/protected", token); response.Code != http.StatusUnauthorized {
		t.Fatalf("request with logged-out token returned %d, want %d", response.Code, http.StatusUnauthorized)
	}
}
