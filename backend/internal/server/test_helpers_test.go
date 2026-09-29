package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"secure-switch-backend/internal/auth"
	"secure-switch-backend/internal/config"
	"secure-switch-backend/internal/device"
	"secure-switch-backend/internal/sshrunner"
	"secure-switch-backend/internal/store"
	"secure-switch-backend/internal/wol"
)

type testHarness struct {
	repository *store.Store
	secret     []byte
	limiter    *auth.LoginAttemptLimiter
	loader     *config.Loader
	monitor    *device.Monitor
}

func newTestHarness(t *testing.T) *testHarness {
	t.Helper()
	repository, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
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
	return &testHarness{
		repository: repository,
		limiter:    auth.NewDefaultLoginAttemptLimiter(),
		loader:     &config.Loader{},
		monitor:    device.NewMonitor(),
	}
}

func (h *testHarness) app() *App {
	return &App{
		Config:   h.loader,
		Store:    h.repository,
		Auth:     &auth.Service{Secret: h.secret, Users: h.repository},
		Limiter:  h.limiter,
		Monitor:  h.monitor,
		Wake:     wol.SendWol,
		Shutdown: sshrunner.RemoteShutdown,
	}
}

func (h *testHarness) router(trustedProxies []string) (*gin.Engine, error) {
	return h.app().Router(trustedProxies)
}

func authenticatedRequest(t *testing.T, router http.Handler, method, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}
