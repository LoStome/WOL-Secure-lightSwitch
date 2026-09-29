package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"secure-switch-backend/internal/auth"
	"secure-switch-backend/internal/config"
	"secure-switch-backend/internal/device"
	"secure-switch-backend/internal/store"
)

func TestAdminMiddlewareProtectsRealEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	workDir := t.TempDir()
	dataDir := filepath.Join(workDir, "data")
	if err := os.Mkdir(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(workDir)
	if err := os.WriteFile(filepath.Join(dataDir, "hosts.yaml"), []byte("- id: test-host\n  name: Test host\n  mac: AA:BB:CC:DD:EE:FF\n"), 0o600); err != nil {
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

	authService := &auth.Service{
		Secret: []byte(strings.Repeat("s", auth.MinimumJWTSecretLength)),
		Users:  repository,
	}
	application := &App{
		Config:  &config.Loader{},
		Store:   repository,
		Auth:    authService,
		Limiter: auth.NewDefaultLoginAttemptLimiter(),
		Monitor: device.NewMonitor(),
		Wake:    func(*config.Host) error { return nil },
		Shutdown: func(*config.Host) error {
			return nil
		},
	}
	router, err := application.Router(nil)
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path string, payload any, cookie *http.Cookie) *httptest.ResponseRecorder {
		t.Helper()
		var body strings.Reader
		if payload != nil {
			encoded, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			body.Reset(string(encoded))
		}
		req := httptest.NewRequest(method, path, &body)
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if cookie != nil {
			req.AddCookie(cookie)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}
	getAuthCookie := func(response *httptest.ResponseRecorder) *http.Cookie {
		t.Helper()
		for _, cookie := range response.Result().Cookies() {
			if cookie.Name == "secure-switch-auth" {
				return cookie
			}
		}
		t.Fatal("login response did not set the authentication cookie")
		return nil
	}

	adminLogin := request(http.MethodPost, "/api/login", map[string]any{
		"email": "admin@example.com", "password": "Strong-Admin-Password-42!",
	}, nil)
	if adminLogin.Code != http.StatusOK {
		t.Fatalf("initial admin login = %d %s, want %d", adminLogin.Code, adminLogin.Body.String(), http.StatusOK)
	}
	adminCookie := getAuthCookie(adminLogin)
	createMember := request(http.MethodPost, "/api/users", map[string]any{
		"email": "member@example.com", "password": "Strong-Member-Password-42!",
		"is_admin": false,
	}, adminCookie)
	if createMember.Code != http.StatusCreated {
		t.Fatalf("create member through admin API = %d %s, want %d", createMember.Code, createMember.Body.String(), http.StatusCreated)
	}
	memberLogin := request(http.MethodPost, "/api/login", map[string]any{
		"email": "member@example.com", "password": "Strong-Member-Password-42!",
	}, nil)
	if memberLogin.Code != http.StatusOK {
		t.Fatalf("member login = %d %s, want %d", memberLogin.Code, memberLogin.Body.String(), http.StatusOK)
	}
	memberCookie := getAuthCookie(memberLogin)

	for _, endpoint := range []struct {
		name, path string
	}{
		{"users", "/api/users"},
		{"login metrics", "/api/metrics/login"},
	} {
		t.Run(endpoint.name, func(t *testing.T) {
			memberResponse := request(http.MethodGet, endpoint.path, nil, memberCookie)
			if memberResponse.Code != http.StatusForbidden {
				t.Errorf("member GET %s = %d %s, want %d", endpoint.path, memberResponse.Code, memberResponse.Body.String(), http.StatusForbidden)
			}
			adminResponse := request(http.MethodGet, endpoint.path, nil, adminCookie)
			if adminResponse.Code != http.StatusOK {
				t.Errorf("admin GET %s = %d %s, want %d", endpoint.path, adminResponse.Code, adminResponse.Body.String(), http.StatusOK)
			}
		})
	}
}

func TestUpdateUserWithoutDevicesPreservesAssignments(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newTestHarness(t)

	user := store.User{
		Email:        "user@example.com",
		PasswordHash: "unused",
		Devices:      []store.UserDevice{{DeviceID: "server-proxmox"}},
	}
	if err := h.repository.DB.Create(&user).Error; err != nil {
		t.Fatalf("create test user: %v", err)
	}

	router := gin.New()
	router.PUT("/users/:id", h.app().HandleUpdateUser)
	request := httptest.NewRequest(
		http.MethodPut,
		"/users/"+strconv.FormatUint(uint64(user.ID), 10),
		bytes.NewBufferString(`{"is_admin":false}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("update returned %d, want %d", response.Code, http.StatusOK)
	}

	var assignments []store.UserDevice
	if err := h.repository.DB.Where("user_id = ?", user.ID).Find(&assignments).Error; err != nil {
		t.Fatalf("load assignments: %v", err)
	}
	if len(assignments) != 1 || assignments[0].DeviceID != "server-proxmox" {
		t.Fatalf("assignments after update = %#v, want the existing assignment preserved", assignments)
	}
}
