package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"secure-switch-backend/internal/store"
)

type fixedUserReader struct{ user *store.User }

func (r fixedUserReader) GetUserByID(id uint) (*store.User, error) {
	return r.user, nil
}

func TestServicesKeepIndependentJWTSecrets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	user := &store.User{ID: 1, Email: "user@example.com"}
	first := &Service{Secret: []byte(strings.Repeat("a", MinimumJWTSecretLength)), Users: fixedUserReader{user}}
	second := &Service{Secret: []byte(strings.Repeat("b", MinimumJWTSecretLength)), Users: fixedUserReader{user}}
	token, err := first.GenerateJWT(user)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.ValidateJWT(token); err != nil {
		t.Fatal(err)
	}
	if _, err := second.ValidateJWT(token); err == nil {
		t.Fatal("token from first service accepted by second service")
	}

	for _, test := range []struct {
		service *Service
		want    int
	}{
		{first, http.StatusNoContent},
		{second, http.StatusUnauthorized},
	} {
		router := gin.New()
		router.GET("/protected", test.service.AuthMiddleware(), func(c *gin.Context) {
			c.Status(http.StatusNoContent)
		})
		request := httptest.NewRequest(http.MethodGet, "/protected", nil)
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != test.want {
			t.Fatalf("auth response = %d, want %d", response.Code, test.want)
		}
	}
}
