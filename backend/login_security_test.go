package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func newTestLoginLimiter(maxFailures, maxTrackedKeys int, window, backoff time.Duration) *loginAttemptLimiter {
	return newLoginAttemptLimiter(maxFailures, window, backoff, maxTrackedKeys, time.Second)
}

func TestLoginLimiterKeysByAccountAndIP(t *testing.T) {
	now := time.Date(2026, time.September, 22, 12, 0, 0, 0, time.UTC)

	t.Run("account across IP addresses", func(t *testing.T) {
		limiter := newTestLoginLimiter(2, 100, time.Minute, time.Minute)
		limiter.RecordFailure("192.0.2.1", "victim@example.com", now)
		limiter.RecordFailure("192.0.2.2", "victim@example.com", now)

		if got := limiter.RetryAfter("192.0.2.3", "victim@example.com", now); got != time.Minute {
			t.Fatalf("account retryAfter = %v, want %v", got, time.Minute)
		}
	})

	t.Run("IP across accounts", func(t *testing.T) {
		limiter := newTestLoginLimiter(2, 100, time.Minute, time.Minute)
		limiter.RecordFailure("192.0.2.1", "first@example.com", now)
		limiter.RecordFailure("192.0.2.1", "second@example.com", now)

		if got := limiter.RetryAfter("192.0.2.1", "third@example.com", now); got != time.Minute {
			t.Fatalf("IP retryAfter = %v, want %v", got, time.Minute)
		}
	})
}

func TestLoginLimiterResetAndExpiration(t *testing.T) {
	now := time.Date(2026, time.September, 22, 12, 0, 0, 0, time.UTC)

	t.Run("successful login clears keys", func(t *testing.T) {
		limiter := newTestLoginLimiter(1, 100, time.Minute, time.Minute)
		limiter.RecordFailure("192.0.2.1", "user@example.com", now)
		limiter.RecordSuccess("192.0.2.1", "user@example.com")

		if got := limiter.RetryAfter("192.0.2.1", "user@example.com", now); got != 0 {
			t.Fatalf("retryAfter after success = %v, want 0", got)
		}
		if got := limiter.Metrics().TrackedKeys; got != 0 {
			t.Fatalf("tracked keys after success = %d, want 0", got)
		}
	})

	t.Run("global cleanup removes expired keys", func(t *testing.T) {
		limiter := newTestLoginLimiter(1, 100, 10*time.Second, time.Second)
		limiter.RecordFailure("192.0.2.1", "user@example.com", now)

		limiter.RetryAfter("192.0.2.99", "other@example.com", now.Add(11*time.Second))
		if got := limiter.Metrics().TrackedKeys; got != 0 {
			t.Fatalf("tracked keys after expiration = %d, want 0", got)
		}
	})
}

func TestLoginLimiterBoundsTrackedKeys(t *testing.T) {
	now := time.Date(2026, time.September, 22, 12, 0, 0, 0, time.UTC)
	limiter := newTestLoginLimiter(5, loginMaxTrackedKeys, time.Hour, time.Minute)

	for index := 0; index < 6_000; index++ {
		limiter.RecordFailure(
			fmt.Sprintf("client-%d", index),
			fmt.Sprintf("user-%d@example.com", index),
			now,
		)
	}

	metrics := limiter.Metrics()
	if metrics.TrackedKeys != loginMaxTrackedKeys {
		t.Fatalf("tracked keys = %d, want %d", metrics.TrackedKeys, loginMaxTrackedKeys)
	}
	if metrics.EvictedKeysTotal == 0 {
		t.Fatal("bounded limiter did not report any evictions")
	}
}

func TestLoginLimiterMetrics(t *testing.T) {
	now := time.Date(2026, time.September, 22, 12, 0, 0, 0, time.UTC)
	limiter := newTestLoginLimiter(1, 100, time.Minute, time.Minute)
	limiter.RecordFailure("192.0.2.1", "user@example.com", now)
	limiter.RecordRateLimited()

	metrics := limiter.Metrics()
	if metrics.FailedAttemptsTotal != 1 || metrics.RateLimitedAttemptsTotal != 1 {
		t.Fatalf("metrics = %+v, want one failure and one rate-limited attempt", metrics)
	}
	if metrics.TrackedKeys != 2 || metrics.EvictedKeysTotal != 0 {
		t.Fatalf("metrics = %+v, want two tracked keys and no evictions", metrics)
	}
}

func TestUnknownAccountUsesDummyBcryptAndIsRateLimited(t *testing.T) {
	cost, err := bcrypt.Cost([]byte(dummyPasswordHash))
	if err != nil {
		t.Fatalf("dummy bcrypt hash is invalid: %v", err)
	}
	if cost != 14 {
		t.Fatalf("dummy bcrypt cost = %d, want 14", cost)
	}

	gin.SetMode(gin.TestMode)
	setupAuthTestDB(t)
	if err := DB.Create(&User{Email: "admin@example.com", PasswordHash: "unused", IsAdmin: true}).Error; err != nil {
		t.Fatalf("create existing admin: %v", err)
	}

	previousLimiter := loginLimiter
	loginLimiter = newTestLoginLimiter(1, 100, time.Minute, time.Minute)
	t.Cleanup(func() { loginLimiter = previousLimiter })

	router := gin.New()
	router.POST("/login", handleLogin)
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
	setupAuthTestDB(t)
	jwtSecret = []byte(strings.Repeat("s", minimumJWTSecretLength))

	admin := User{Email: "admin@example.com", PasswordHash: "unused", IsAdmin: true}
	standard := User{Email: "user@example.com", PasswordHash: "unused"}
	if err := DB.Create(&admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}
	if err := DB.Create(&standard).Error; err != nil {
		t.Fatalf("create standard user: %v", err)
	}

	previousLimiter := loginLimiter
	loginLimiter = newTestLoginLimiter(1, 100, time.Minute, time.Minute)
	loginLimiter.RecordFailure("192.0.2.1", "user@example.com", time.Now())
	loginLimiter.RecordRateLimited()
	t.Cleanup(func() { loginLimiter = previousLimiter })

	router, err := newRouter(nil)
	if err != nil {
		t.Fatalf("create router: %v", err)
	}
	adminToken, err := GenerateJWT(&admin)
	if err != nil {
		t.Fatalf("generate admin token: %v", err)
	}
	userToken, err := GenerateJWT(&standard)
	if err != nil {
		t.Fatalf("generate user token: %v", err)
	}

	adminResponse := authenticatedRequest(t, router, http.MethodGet, "/api/metrics/login", adminToken)
	if adminResponse.Code != http.StatusOK {
		t.Fatalf("admin metrics request returned %d, want %d", adminResponse.Code, http.StatusOK)
	}
	var metrics loginMetricsSnapshot
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
	setupAuthTestDB(t)
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	for _, email := range []string{"first@example.com", "second@example.com"} {
		if err := DB.Create(&User{Email: email, PasswordHash: string(passwordHash), IsAdmin: true}).Error; err != nil {
			t.Fatalf("create %s: %v", email, err)
		}
	}

	previousLimiter := loginLimiter
	t.Cleanup(func() { loginLimiter = previousLimiter })

	newLoginRouter := func(trustedProxies []string) *gin.Engine {
		router := gin.New()
		if err := router.SetTrustedProxies(trustedProxies); err != nil {
			t.Fatalf("set trusted proxies: %v", err)
		}
		router.POST("/login", handleLogin)
		return router
	}

	t.Run("trusted proxy separates forwarded clients", func(t *testing.T) {
		loginLimiter = newTestLoginLimiter(1, 100, time.Minute, time.Minute)
		router := newLoginRouter([]string{"192.0.2.10"})
		first := performLoginRequest(router, `{"email":"first@example.com","password":"wrong"}`, "192.0.2.10:1234", "203.0.113.1")
		second := performLoginRequest(router, `{"email":"second@example.com","password":"wrong"}`, "192.0.2.10:1234", "203.0.113.2")
		if first.Code != http.StatusUnauthorized || second.Code != http.StatusUnauthorized {
			t.Fatalf("trusted proxy statuses = %d, %d; want 401, 401", first.Code, second.Code)
		}
	})

	t.Run("untrusted client cannot spoof forwarded address", func(t *testing.T) {
		loginLimiter = newTestLoginLimiter(1, 100, time.Minute, time.Minute)
		router := newLoginRouter([]string{"192.0.2.99"})
		first := performLoginRequest(router, `{"email":"first@example.com","password":"wrong"}`, "192.0.2.10:1234", "203.0.113.1")
		second := performLoginRequest(router, `{"email":"second@example.com","password":"wrong"}`, "192.0.2.10:1234", "203.0.113.2")
		if first.Code != http.StatusUnauthorized || second.Code != http.StatusTooManyRequests {
			t.Fatalf("untrusted client statuses = %d, %d; want 401, 429", first.Code, second.Code)
		}
	})
}

func TestParseTrustedProxies(t *testing.T) {
	proxies, err := parseTrustedProxies(" 127.0.0.1, 10.0.0.0/8, ::1 ")
	if err != nil {
		t.Fatalf("parse valid trusted proxies: %v", err)
	}
	want := []string{"127.0.0.1", "10.0.0.0/8", "::1"}
	if fmt.Sprint(proxies) != fmt.Sprint(want) {
		t.Fatalf("trusted proxies = %v, want %v", proxies, want)
	}

	for _, value := range []string{"not-an-address", "127.0.0.1,,::1", "0.0.0.0/0", "::/0"} {
		if _, err := parseTrustedProxies(value); err == nil {
			t.Errorf("parseTrustedProxies(%q) succeeded, want error", value)
		}
	}
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
