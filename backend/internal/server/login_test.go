package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"secure-switch-backend/internal/auth"
	"secure-switch-backend/internal/store"
)

func newTestLoginLimiter(maxFailures, maxTrackedKeys int, window, backoff time.Duration) *auth.LoginAttemptLimiter {
	return auth.NewLoginAttemptLimiter(maxFailures, window, backoff, maxTrackedKeys, time.Second)
}

func TestUnknownAccountUsesDummyBcryptAndIsRateLimited(t *testing.T) {
	cost, err := bcrypt.Cost([]byte(auth.DummyPasswordHash))
	if err != nil {
		t.Fatalf("dummy bcrypt hash is invalid: %v", err)
	}
	if cost != 14 {
		t.Fatalf("dummy bcrypt cost = %d, want 14", cost)
	}

	gin.SetMode(gin.TestMode)
	h := newTestHarness(t)
	if err := h.repository.DB.Create(&store.User{Email: "admin@example.com", PasswordHash: "unused", IsAdmin: true}).Error; err != nil {
		t.Fatalf("create existing admin: %v", err)
	}

	h.limiter = newTestLoginLimiter(1, 100, time.Minute, time.Minute)

	router := gin.New()
	router.POST("/login", h.app().HandleLogin)
	body := `{"email":"missing@example.com","password":"wrong-password"}`

	first := performLoginRequest(router, body, "192.0.2.1:1234", "")
	if first.Code != http.StatusUnauthorized || !strings.Contains(first.Body.String(), "Invalid email or password") {
		t.Fatalf("unknown-account login = %d %q, want generic 401", first.Code, first.Body.String())
	}
	second := performLoginRequest(router, body, "192.0.2.1:1234", "")
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("repeated unknown-account login = %d, want %d", second.Code, http.StatusTooManyRequests)
	}
}

func TestLoginMetricsEndpointRequiresAdministrator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newTestHarness(t)
	h.secret = []byte(strings.Repeat("s", auth.MinimumJWTSecretLength))

	admin := store.User{Email: "admin@example.com", PasswordHash: "unused", IsAdmin: true}
	standard := store.User{Email: "user@example.com", PasswordHash: "unused"}
	if err := h.repository.DB.Create(&admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}
	if err := h.repository.DB.Create(&standard).Error; err != nil {
		t.Fatalf("create standard user: %v", err)
	}

	h.limiter = newTestLoginLimiter(1, 100, time.Minute, time.Minute)
	h.limiter.RecordFailure("192.0.2.1", "user@example.com", time.Now())
	h.limiter.RecordRateLimited()

	router, err := h.router(nil)
	if err != nil {
		t.Fatalf("create router: %v", err)
	}
	adminToken, err := h.app().Auth.GenerateJWT(&admin)
	if err != nil {
		t.Fatalf("generate admin token: %v", err)
	}
	userToken, err := h.app().Auth.GenerateJWT(&standard)
	if err != nil {
		t.Fatalf("generate user token: %v", err)
	}

	adminResponse := authenticatedRequest(t, router, http.MethodGet, "/api/metrics/login", adminToken)
	if adminResponse.Code != http.StatusOK {
		t.Fatalf("admin metrics request returned %d, want %d", adminResponse.Code, http.StatusOK)
	}
	var metrics auth.LoginMetricsSnapshot
	if err := json.Unmarshal(adminResponse.Body.Bytes(), &metrics); err != nil {
		t.Fatalf("decode metrics response: %v", err)
	}
	if metrics.FailedAttemptsTotal != 1 || metrics.RateLimitedAttemptsTotal != 1 || metrics.TrackedKeys != 2 {
		t.Fatalf("metrics response = %+v, want current limiter snapshot", metrics)
	}

	userResponse := authenticatedRequest(t, router, http.MethodGet, "/api/metrics/login", userToken)
	if userResponse.Code != http.StatusForbidden {
		t.Fatalf("standard-user metrics request returned %d, want %d", userResponse.Code, http.StatusForbidden)
	}
}

func TestLoginRateLimitHonorsOnlyTrustedForwardedFor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newTestHarness(t)
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	for _, email := range []string{"first@example.com", "second@example.com"} {
		if err := h.repository.DB.Create(&store.User{Email: email, PasswordHash: string(passwordHash), IsAdmin: true}).Error; err != nil {
			t.Fatalf("create %s: %v", email, err)
		}
	}

	newLoginRouter := func(trustedProxies []string) *gin.Engine {
		router := gin.New()
		if err := router.SetTrustedProxies(trustedProxies); err != nil {
			t.Fatalf("set trusted proxies: %v", err)
		}
		router.POST("/login", h.app().HandleLogin)
		return router
	}

	t.Run("trusted proxy separates forwarded clients", func(t *testing.T) {
		h.limiter = newTestLoginLimiter(1, 100, time.Minute, time.Minute)
		router := newLoginRouter([]string{"192.0.2.10"})
		first := performLoginRequest(router, `{"email":"first@example.com","password":"wrong"}`, "192.0.2.10:1234", "203.0.113.1")
		second := performLoginRequest(router, `{"email":"second@example.com","password":"wrong"}`, "192.0.2.10:1234", "203.0.113.2")
		if first.Code != http.StatusUnauthorized || second.Code != http.StatusUnauthorized {
			t.Fatalf("trusted proxy statuses = %d, %d; want 401, 401", first.Code, second.Code)
		}
	})

	t.Run("untrusted client cannot spoof forwarded address", func(t *testing.T) {
		h.limiter = newTestLoginLimiter(1, 100, time.Minute, time.Minute)
		router := newLoginRouter([]string{"192.0.2.99"})
		first := performLoginRequest(router, `{"email":"first@example.com","password":"wrong"}`, "192.0.2.10:1234", "203.0.113.1")
		second := performLoginRequest(router, `{"email":"second@example.com","password":"wrong"}`, "192.0.2.10:1234", "203.0.113.2")
		if first.Code != http.StatusUnauthorized || second.Code != http.StatusTooManyRequests {
			t.Fatalf("untrusted client statuses = %d, %d; want 401, 429", first.Code, second.Code)
		}
	})
}

func performLoginRequest(router http.Handler, body, remoteAddress, forwardedFor string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = remoteAddress
	if forwardedFor != "" {
		request.Header.Set("X-Forwarded-For", forwardedFor)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestLoginRateLimitsRepeatedInvalidPasswords(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newTestHarness(t)

	passwordHash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash test password: %v", err)
	}
	if err := h.repository.DB.Create(&store.User{
		Email:        "user@example.com",
		PasswordHash: string(passwordHash),
		IsAdmin:      true,
	}).Error; err != nil {
		t.Fatalf("create test user: %v", err)
	}

	h.limiter = auth.NewLoginAttemptLimiter(2, time.Minute, time.Minute, auth.LoginMaxTrackedKeys, auth.LoginCleanupInterval)

	router := gin.New()
	router.POST("/login", h.app().HandleLogin)
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
