package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouterAddsSecurityHeaders(t *testing.T) {
	router, err := newRouter(nil)
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
