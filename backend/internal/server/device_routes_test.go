package server

import (
	"encoding/json"
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

func TestRouterRestrictsDevicesToUserAssignments(t *testing.T) {
	gin.SetMode(gin.TestMode)
	workDir := t.TempDir()
	dataDir := filepath.Join(workDir, "data")
	if err := os.Mkdir(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(workDir)
	hostsYAML := "- id: assigned-host\n  name: Assigned host\n  mac: AA:BB:CC:DD:EE:01\n" +
		"- id: unassigned-host\n  name: Unassigned host\n  mac: AA:BB:CC:DD:EE:02\n"
	if err := os.WriteFile(filepath.Join(dataDir, "hosts.yaml"), []byte(hostsYAML), 0o600); err != nil {
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

	wakeCalls := make([]string, 0, 2)
	shutdownCalls := make([]string, 0, 2)
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
		Wake: func(host *config.Host) error {
			wakeCalls = append(wakeCalls, host.ID)
			return nil
		},
		Shutdown: func(host *config.Host) error {
			shutdownCalls = append(shutdownCalls, host.ID)
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
		"is_admin": false, "devices": []string{"assigned-host"},
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

	hostResponse := request(http.MethodGet, "/api/hosts", nil, memberCookie)
	if hostResponse.Code != http.StatusOK {
		t.Fatalf("member hosts = %d %s, want %d", hostResponse.Code, hostResponse.Body.String(), http.StatusOK)
	}
	var hosts []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(hostResponse.Body.Bytes(), &hosts); err != nil {
		t.Fatalf("decode member hosts: %v", err)
	}
	if len(hosts) != 1 || hosts[0].ID != "assigned-host" {
		t.Fatalf("member hosts = %+v, want only assigned-host", hosts)
	}

	for _, action := range []struct {
		name, path string
		calls      *[]string
	}{
		{"wol", "/api/wol", &wakeCalls},
		{"shutdown", "/api/shutdown", &shutdownCalls},
	} {
		t.Run(action.name, func(t *testing.T) {
			allowed := request(http.MethodPost, action.path+"/assigned-host", nil, memberCookie)
			if allowed.Code != http.StatusOK {
				t.Fatalf("assigned %s = %d %s, want %d", action.name, allowed.Code, allowed.Body.String(), http.StatusOK)
			}
			if len(*action.calls) != 1 || (*action.calls)[0] != "assigned-host" {
				t.Fatalf("assigned %s calls = %v, want [assigned-host]", action.name, *action.calls)
			}

			denied := request(http.MethodPost, action.path+"/unassigned-host", nil, memberCookie)
			if denied.Code != http.StatusForbidden {
				t.Errorf("unassigned %s = %d %s, want %d", action.name, denied.Code, denied.Body.String(), http.StatusForbidden)
			}
			if len(*action.calls) != 1 {
				t.Errorf("unassigned %s invoked action stub; calls = %v", action.name, *action.calls)
			}
		})
	}
}
