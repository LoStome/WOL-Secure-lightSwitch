package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestUpdateUserWithoutDevicesPreservesAssignments(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupAuthTestDB(t)

	user := User{
		Email:        "user@example.com",
		PasswordHash: "unused",
		Devices:      []UserDevice{{DeviceID: "server-proxmox"}},
	}
	if err := DB.Create(&user).Error; err != nil {
		t.Fatalf("create test user: %v", err)
	}

	router := gin.New()
	router.PUT("/users/:id", handleUpdateUser)
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

	var assignments []UserDevice
	if err := DB.Where("user_id = ?", user.ID).Find(&assignments).Error; err != nil {
		t.Fatalf("load assignments: %v", err)
	}
	if len(assignments) != 1 || assignments[0].DeviceID != "server-proxmox" {
		t.Fatalf("assignments after update = %#v, want the existing assignment preserved", assignments)
	}
}

func TestSEC10ValidationRules(t *testing.T) {
	t.Run("email", func(t *testing.T) {
		if got, err := validateEmail(" user@example.com "); err != nil || got != "user@example.com" {
			t.Fatalf("validateEmail() = %q, %v; want trimmed valid email", got, err)
		}
		for _, email := range []string{"not-an-email", "Display Name <user@example.com>", strings.Repeat("a", maxEmailBytes+1)} {
			if _, err := validateEmail(email); err == nil {
				t.Errorf("validateEmail(%q) succeeded, want an error", email)
			}
		}
	})

	t.Run("password", func(t *testing.T) {
		if err := validatePassword(strings.Repeat("a", minimumPasswordLength), true); err != nil {
			t.Fatalf("minimum password rejected: %v", err)
		}
		for _, password := range []string{"short", strings.Repeat("a", maxPasswordBytes+1)} {
			if err := validatePassword(password, true); err == nil {
				t.Errorf("validatePassword(%q) succeeded, want an error", password)
			}
		}
	})

	t.Run("device IDs", func(t *testing.T) {
		configured := map[string]struct{}{"server-proxmox": {}}
		if err := validateDeviceIDs([]string{"server-proxmox"}, configured); err != nil {
			t.Fatalf("configured device ID rejected: %v", err)
		}
		for _, deviceIDs := range [][]string{
			{"unknown-device"},
			{"server-proxmox", "server-proxmox"},
			{"invalid/device"},
		} {
			if err := validateDeviceIDs(deviceIDs, configured); err == nil {
				t.Errorf("validateDeviceIDs(%v) succeeded, want an error", deviceIDs)
			}
		}
	})
}

func TestRequestBodyLimit(t *testing.T) {
	router := gin.New()
	router.Use(requestBodyLimitMiddleware())
	router.POST("/", func(c *gin.Context) {
		var request map[string]string
		if err := c.ShouldBindJSON(&request); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		c.Status(http.StatusNoContent)
	})

	body := append([]byte(`{"value":"`), bytes.Repeat([]byte("a"), int(maxRequestBodyBytes))...)
	body = append(body, '"', '}')
	request := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("oversized request returned %d, want %d", response.Code, http.StatusBadRequest)
	}
}
