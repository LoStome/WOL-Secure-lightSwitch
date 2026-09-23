package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"secure-switch-backend/internal/auth"
	"secure-switch-backend/internal/store"
)

const sec12AuthCookieName = "secure-switch-auth"

func TestCookieAuthenticationDoesNotExposeToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newTestHarness(t)
	h.secret = []byte(strings.Repeat("s", auth.MinimumJWTSecretLength))

	passwordHash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash test password: %v", err)
	}
	if err := h.repository.DB.Create(&store.User{
		Email:        "sec12@example.com",
		PasswordHash: string(passwordHash),
	}).Error; err != nil {
		t.Fatalf("create test user: %v", err)
	}

	router := gin.New()
	router.POST("/login", h.app().HandleLogin)
	router.GET("/protected", h.app().Auth.AuthMiddleware(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	router.POST("/logout", h.app().Auth.AuthMiddleware(), h.app().HandleLogout)

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

}

func TestLogoutRevokesCurrentToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newTestHarness(t)
	h.secret = []byte(strings.Repeat("s", auth.MinimumJWTSecretLength))

	user := store.User{Email: "user@example.com", PasswordHash: "unused"}
	if err := h.repository.DB.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	token, err := h.app().Auth.GenerateJWT(&user)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	router := gin.New()
	router.POST("/logout", h.app().Auth.AuthMiddleware(), h.app().HandleLogout)
	router.GET("/protected", h.app().Auth.AuthMiddleware(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	if response := authenticatedRequest(t, router, http.MethodPost, "/logout", token); response.Code != http.StatusNoContent {
		t.Fatalf("logout returned %d, want %d", response.Code, http.StatusNoContent)
	}
	if response := authenticatedRequest(t, router, http.MethodGet, "/protected", token); response.Code != http.StatusUnauthorized {
		t.Fatalf("request with logged-out token returned %d, want %d", response.Code, http.StatusUnauthorized)
	}
}
