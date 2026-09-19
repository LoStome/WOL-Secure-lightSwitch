package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouterDoesNotAllowCrossOriginRequests(t *testing.T) {
	router := newRouter()
	request := httptest.NewRequest(http.MethodOptions, "/api/login", nil)
	request.Header.Set("Origin", "https://attacker.example")
	request.Header.Set("Access-Control-Request-Method", http.MethodPost)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if origin := response.Header().Get("Access-Control-Allow-Origin"); origin != "" {
		t.Fatalf("cross-origin preflight allowed origin %q, want no CORS access", origin)
	}
}
