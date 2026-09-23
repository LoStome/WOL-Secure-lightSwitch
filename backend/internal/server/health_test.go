package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

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
