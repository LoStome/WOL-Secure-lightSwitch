package main

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"secure-switch-backend/internal/auth"
	"secure-switch-backend/internal/config"
	"secure-switch-backend/internal/device"
	"secure-switch-backend/internal/server"
	"secure-switch-backend/internal/sshrunner"
	"secure-switch-backend/internal/store"
	"secure-switch-backend/internal/wol"
)

// Legacy test names map to explicit application instances during the package move.
var DB *gorm.DB
var jwtSecret []byte
var loginLimiter = auth.NewDefaultLoginAttemptLimiter()
var hostLoader = &config.Loader{}

type Host = config.Host
type User = store.User
type UserDevice = store.UserDevice
type Claims = auth.Claims
type loginAttemptLimiter = auth.LoginAttemptLimiter
type loginMetricsSnapshot = auth.LoginMetricsSnapshot

const minimumJWTSecretLength = auth.MinimumJWTSecretLength
const loginMaxTrackedKeys = auth.LoginMaxTrackedKeys
const loginCleanupInterval = auth.LoginCleanupInterval
const dummyPasswordHash = auth.DummyPasswordHash
const maxRequestBodyBytes = server.MaxRequestBodyBytes
const maxEmailBytes = server.MaxEmailBytes
const minimumPasswordLength = server.MinimumPasswordLength
const maxPasswordBytes = server.MaxPasswordBytes

var ErrInitialAdminExists = store.ErrInitialAdminExists
var ErrLastAdministrator = store.ErrLastAdministrator

func currentStore() *store.Store   { return &store.Store{DB: DB} }
func sqliteDSN(path string) string { return store.SQLiteDSN(path) }
func CreateInitialAdmin(email, passwordHash string) error {
	return currentStore().CreateInitialAdmin(email, passwordHash)
}
func GetAdminCount() (int64, error) { return currentStore().GetAdminCount() }
func UpdateUser(userID uint, passwordHash *string, isAdmin *bool, deviceIDs *[]string) error {
	return currentStore().UpdateUser(userID, passwordHash, isAdmin, deviceIDs)
}
func DeleteUser(userID uint) error { return currentStore().DeleteUser(userID) }

func loadJWTSecret() ([]byte, error)         { return auth.LoadJWTSecret() }
func GenerateJWT(user *User) (string, error) { return testApp().Auth.GenerateJWT(user) }
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) { testApp().Auth.AuthMiddleware()(c) }
}
func AdminMiddleware() gin.HandlerFunc { return auth.AdminMiddleware() }

func newLoginAttemptLimiter(maxFailures int, window, backoff time.Duration, maxTrackedKeys int, cleanupInterval time.Duration) *loginAttemptLimiter {
	return auth.NewLoginAttemptLimiter(maxFailures, window, backoff, maxTrackedKeys, cleanupInterval)
}
func parseTrustedProxies(value string) ([]string, error) { return auth.ParseTrustedProxies(value) }

func LoadHosts() ([]Host, error)             { return hostLoader.LoadHosts() }
func parseHosts(data []byte) ([]Host, error) { return config.ParseHosts(data) }
func IsOnline(ip string) bool                { return device.IsOnline(ip) }
func StartPingManager(ctx context.Context)   { hostStates.StartPingManager(ctx, LoadHosts) }

var hostStates = device.NewMonitor()

func testApp() *server.App {
	repository := currentStore()
	return &server.App{
		Config:   hostLoader,
		Store:    repository,
		Auth:     &auth.Service{Secret: jwtSecret, Users: repository},
		Limiter:  loginLimiter,
		Monitor:  hostStates,
		Wake:     wol.SendWol,
		Shutdown: sshrunner.RemoteShutdown,
	}
}
func newRouter(trustedProxies []string) (*gin.Engine, error) { return testApp().Router(trustedProxies) }
func newHTTPServer(handler http.Handler, address string) *http.Server {
	return server.NewHTTPServer(handler, address)
}
func handleLogin(c *gin.Context)                  { testApp().HandleLogin(c) }
func handleLogout(c *gin.Context)                 { testApp().HandleLogout(c) }
func handleUpdateUser(c *gin.Context)             { testApp().HandleUpdateUser(c) }
func requestBodyLimitMiddleware() gin.HandlerFunc { return server.RequestBodyLimitMiddleware() }
func validateEmail(email string) (string, error)  { return server.ValidateEmail(email) }
func validatePassword(password string, requireMinimum bool) error {
	return server.ValidatePassword(password, requireMinimum)
}
func validateDeviceIDs(deviceIDs []string, configured map[string]struct{}) error {
	return server.ValidateDeviceIDs(deviceIDs, configured)
}
