package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"secure-switch-backend/internal/auth"
	"secure-switch-backend/internal/config"
)

func TestFirstStartWithNoDevicesIsHealthy(t *testing.T) {
	workDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(workDir, "data"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(workDir)
	if err := config.EnsureInitialHosts(filepath.Join("data", "hosts.yaml")); err != nil {
		t.Fatal(err)
	}
	h := newTestHarness(t)
	h.secret = []byte(strings.Repeat("s", auth.MinimumJWTSecretLength))
	router, err := h.router(nil)
	if err != nil {
		t.Fatal(err)
	}
	health := httptest.NewRecorder()
	router.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if health.Code != http.StatusNoContent {
		t.Fatalf("health = %d, want 204", health.Code)
	}

	login := httptest.NewRecorder()
	router.ServeHTTP(login, httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"email":"admin@example.com","password":"Strong-Admin-Password-42!"}`)))
	if login.Code != http.StatusOK {
		t.Fatalf("initial admin login = %d: %s", login.Code, login.Body.String())
	}
	request := httptest.NewRequest(http.MethodGet, "/api/hosts", nil)
	for _, cookie := range login.Result().Cookies() {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("hosts status = %d: %s", response.Code, response.Body.String())
	}
	var hosts []Host
	if err := json.Unmarshal(response.Body.Bytes(), &hosts); err != nil || hosts == nil || len(hosts) != 0 {
		t.Fatalf("hosts response = %q, decoded = %+v, err = %v", response.Body.String(), hosts, err)
	}
}

func TestHealthzChecksConfigurationAndDatabase(t *testing.T) {
	workDir := filepath.Join(t.TempDir(), "app")
	configDir := filepath.Join(workDir, "data")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("create config directory: %v", err)
	}
	t.Chdir(workDir)

	configPath := filepath.Join(configDir, "hosts.yaml")
	validConfig := "- id: test-host\n  name: Test host\n  mac: AA:BB:CC:DD:EE:FF\n"
	if err := os.WriteFile(configPath, []byte(validConfig), 0o600); err != nil {
		t.Fatalf("write test config: %v", err)
	}

	h := newTestHarness(t)
	sqlDB, err := h.repository.DB.DB()
	if err != nil {
		t.Fatalf("get test database handle: %v", err)
	}

	router, err := h.router(nil)
	if err != nil {
		t.Fatalf("create router: %v", err)
	}
	requestHealth := func() *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}

	response := requestHealth()
	if response.Code != http.StatusNoContent || response.Body.Len() != 0 {
		t.Fatalf("healthy response = %d %q, want 204 with no body", response.Code, response.Body.String())
	}

	if err := os.WriteFile(configPath, []byte("invalid: [yaml"), 0o600); err != nil {
		t.Fatalf("write invalid test config: %v", err)
	}
	response = requestHealth()
	if response.Code != http.StatusServiceUnavailable || response.Body.Len() != 0 {
		t.Fatalf("invalid config response = %d %q, want 503 with no body", response.Code, response.Body.String())
	}
	if hosts, err := h.loader.LoadHosts(); err != nil || len(hosts) != 1 || hosts[0].ID != "test-host" {
		t.Fatalf("hosts during invalid config = %+v, err = %v; want last valid host", hosts, err)
	}

	if err := os.WriteFile(configPath, []byte(validConfig), 0o600); err != nil {
		t.Fatalf("restore test config: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close test database: %v", err)
	}
	response = requestHealth()
	if response.Code != http.StatusServiceUnavailable || response.Body.Len() != 0 {
		t.Fatalf("unavailable database response = %d %q, want 503 with no body", response.Code, response.Body.String())
	}
}
