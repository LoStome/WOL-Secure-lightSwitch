package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

const sec12AuthCookieName = "secure-switch-auth"

func TestSEC12UsesCookieAuthenticationWithoutPersistentBrowserStorage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupAuthTestDB(t)
	jwtSecret = []byte(strings.Repeat("s", minimumJWTSecretLength))

	passwordHash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash test password: %v", err)
	}
	if err := DB.Create(&User{
		Email:        "sec12@example.com",
		PasswordHash: string(passwordHash),
	}).Error; err != nil {
		t.Fatalf("create test user: %v", err)
	}

	router := gin.New()
	router.POST("/login", handleLogin)
	router.GET("/protected", AuthMiddleware(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	router.POST("/logout", AuthMiddleware(), handleLogout)

	loginRequest := httptest.NewRequest(
		http.MethodPost,
		"/login",
		bytes.NewBufferString(`{"email":"sec12@example.com","password":"correct-password"}`),
	)
	loginRequest.Header.Set("Content-Type", "application/json")
	loginResponse := httptest.NewRecorder()
	router.ServeHTTP(loginResponse, loginRequest)
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login returned %d, want %d", loginResponse.Code, http.StatusOK)
	}

	var loginBody map[string]json.RawMessage
	if err := json.Unmarshal(loginResponse.Body.Bytes(), &loginBody); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if _, exists := loginBody["token"]; exists {
		t.Fatal("login response exposes the token to JavaScript")
	}

	var authCookie *http.Cookie
	for _, cookie := range loginResponse.Result().Cookies() {
		if cookie.Name == sec12AuthCookieName {
			authCookie = cookie
			break
		}
	}
	if authCookie == nil {
		t.Fatalf("login did not set %q cookie", sec12AuthCookieName)
	}
	if !authCookie.HttpOnly || !authCookie.Secure || authCookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("auth cookie flags = HttpOnly:%t Secure:%t SameSite:%d, want HttpOnly, Secure, SameSite=Strict", authCookie.HttpOnly, authCookie.Secure, authCookie.SameSite)
	}

	protectedRequest := httptest.NewRequest(http.MethodGet, "/protected", nil)
	protectedRequest.AddCookie(authCookie)
	protectedResponse := httptest.NewRecorder()
	router.ServeHTTP(protectedResponse, protectedRequest)
	if protectedResponse.Code != http.StatusNoContent {
		t.Fatalf("cookie-authenticated request returned %d, want %d", protectedResponse.Code, http.StatusNoContent)
	}

	for _, sourcePath := range []string{
		filepath.Join("..", "frontend", "src", "App.tsx"),
		filepath.Join("..", "frontend", "src", "services", "api.ts"),
	} {
		source, err := os.ReadFile(sourcePath)
		if err != nil {
			t.Fatalf("read %s: %v", sourcePath, err)
		}
		if strings.Contains(string(source), "localStorage") {
			t.Fatalf("frontend persists authentication data in localStorage: %s", sourcePath)
		}
	}
}
