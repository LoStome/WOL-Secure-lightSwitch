package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRouterDoesNotAllowCrossOriginRequests(t *testing.T) {
	h := newTestHarness(t)
	router, err := h.router(nil)
	if err != nil {
		t.Fatalf("create router: %v", err)
	}
	request := httptest.NewRequest(http.MethodOptions, "/api/login", nil)
	request.Header.Set("Origin", "https://attacker.example")
	request.Header.Set("Access-Control-Request-Method", http.MethodPost)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if origin := response.Header().Get("Access-Control-Allow-Origin"); origin != "" {
		t.Fatalf("cross-origin preflight allowed origin %q, want no CORS access", origin)
	}
}

func TestRouterAddsSecurityHeaders(t *testing.T) {
	h := newTestHarness(t)
	router, err := h.router(nil)
	if err != nil {
		t.Fatalf("create router: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/security-header-check", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	wantHeaders := map[string]string{
		"Content-Security-Policy":   "default-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; object-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'",
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
		"Referrer-Policy":           "no-referrer",
	}

	for name, want := range wantHeaders {
		if got := response.Header().Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestRequestBodyLimit(t *testing.T) {
	router := gin.New()
	router.Use(RequestBodyLimitMiddleware())
	router.POST("/", func(c *gin.Context) {
		var request map[string]string
		if err := c.ShouldBindJSON(&request); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		c.Status(http.StatusNoContent)
	})

	body := append([]byte(`{"value":"`), bytes.Repeat([]byte("a"), int(MaxRequestBodyBytes))...)
	body = append(body, '"', '}')
	request := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("oversized request returned %d, want %d", response.Code, http.StatusBadRequest)
	}
}
