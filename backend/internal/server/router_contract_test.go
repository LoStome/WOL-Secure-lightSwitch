package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"secure-switch-backend/internal/auth"
	"secure-switch-backend/internal/config"
	"secure-switch-backend/internal/device"
	"secure-switch-backend/internal/store"
)

func TestRouterPreservesRoutesAndDeviceResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	workDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(workDir, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(workDir)
	if err := os.WriteFile(filepath.Join("data", "hosts.yaml"), []byte("- id: test-host\n  name: Test host\n  mac: AA:BB:CC:DD:EE:FF\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	repository, err := store.Open(filepath.Join(workDir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.Migrate(); err != nil {
		t.Fatal(err)
	}
	sqlDB, err := repository.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := repository.CreateUser("admin@example.com", "unused", true, nil); err != nil {
		t.Fatal(err)
	}
	admin, err := repository.GetUserByEmail("admin@example.com")
	if err != nil {
		t.Fatal(err)
	}
	authService := &auth.Service{Secret: []byte(strings.Repeat("s", auth.MinimumJWTSecretLength)), Users: repository}
	token, err := authService.GenerateJWT(admin)
	if err != nil {
		t.Fatal(err)
	}
	wakeCalls, shutdownCalls := 0, 0
	application := &App{
		Config:  &config.Loader{},
		Store:   repository,
		Auth:    authService,
		Limiter: auth.NewDefaultLoginAttemptLimiter(),
		Monitor: device.NewMonitor(),
		Wake: func(host *config.Host) error {
			wakeCalls++
			if host.ID != "test-host" {
				t.Errorf("wake host = %q", host.ID)
			}
			return nil
		},
		Shutdown: func(host *config.Host) error {
			shutdownCalls++
			if host.ID != "test-host" {
				t.Errorf("shutdown host = %q", host.ID)
			}
			return nil
		},
	}
	router, err := application.Router(nil)
	if err != nil {
		t.Fatal(err)
	}
	wantRoutes := map[string]bool{
		"GET /healthz": false, "POST /api/login": false, "GET /api/setup": false,
		"POST /api/logout": false, "GET /api/session": false, "GET /api/ping": false,
		"GET /api/hosts": false, "POST /api/wol/:id": false, "POST /api/shutdown/:id": false,
		"GET /api/users": false, "POST /api/users": false, "PUT /api/users/:id": false,
		"DELETE /api/users/:id": false, "GET /api/metrics/login": false,
	}
	for _, route := range router.Routes() {
		key := route.Method + " " + route.Path
		if _, exists := wantRoutes[key]; !exists {
			t.Errorf("unexpected route %s", key)
		}
		wantRoutes[key] = true
	}
	for route, found := range wantRoutes {
		if !found {
			t.Errorf("missing route %s", route)
		}
	}
	request := func(method, path string, authorized bool) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, nil)
		if authorized {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, r)
		return response
	}
	for _, scenario := range []struct {
		method, path string
		authorized   bool
		status       int
		body         string
	}{
		{http.MethodGet, "/healthz", false, http.StatusNoContent, ""},
		{http.MethodGet, "/api/setup", false, http.StatusOK, `{"needs_setup":false}`},
		{http.MethodGet, "/api/hosts", false, http.StatusUnauthorized, `{"error":"Authorization header is required"}`},
		{http.MethodGet, "/api/ping", true, http.StatusOK, `{"message":"pong"}`},
		{http.MethodPost, "/api/wol/test-host", true, http.StatusOK, `{"message":"Magic Packet sent successfully to Test host"}`},
		{http.MethodPost, "/api/shutdown/test-host", true, http.StatusOK, `{"message":"Shutdown command received from Test host"}`},
	} {
		response := request(scenario.method, scenario.path, scenario.authorized)
		if response.Code != scenario.status || strings.TrimSpace(response.Body.String()) != scenario.body {
			t.Errorf("%s %s = %d %q, want %d %q", scenario.method, scenario.path, response.Code, response.Body.String(), scenario.status, scenario.body)
		}
	}
	if wakeCalls != 1 || shutdownCalls != 1 {
		t.Fatalf("action calls = wake %d, shutdown %d; want one each", wakeCalls, shutdownCalls)
	}
}
