package auth

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"secure-switch-backend/internal/store"
)

func TestAdminMiddlewareUsesCurrentDatabaseRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
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
	service := &Service{Secret: []byte(strings.Repeat("s", MinimumJWTSecretLength)), Users: repository}

	admin := store.User{Email: "admin@example.com", PasswordHash: "unused", IsAdmin: true}
	if err := repository.DB.Create(&admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}
	token, err := service.GenerateJWT(&admin)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	router := gin.New()
	router.GET("/admin", service.AuthMiddleware(), AdminMiddleware(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	request := func() *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/admin", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}
	if response := request(); response.Code != http.StatusNoContent {
		t.Fatalf("admin request before demotion returned %d, want %d", response.Code, http.StatusNoContent)
	}

	if err := repository.DB.Model(&admin).Update("is_admin", false).Error; err != nil {
		t.Fatalf("demote admin: %v", err)
	}
	if response := request(); response.Code != http.StatusForbidden {
		t.Fatalf("admin request after demotion returned %d, want %d", response.Code, http.StatusForbidden)
	}
}
