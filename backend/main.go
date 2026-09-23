package main

import (
	"context"
	cryptoRand "crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"net/mail"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Host struct {
	ID             string   `yaml:"id"`
	Name           string   `yaml:"name"`
	MAC            string   `yaml:"mac"`
	IP             string   `yaml:"ip"`
	User           string   `yaml:"user" json:"-"`
	PasswordFile   string   `yaml:"password_file" json:"-"`
	KeyPath        string   `yaml:"key_path" json:"-"`
	Cmd            string   `yaml:"cmd" json:"-"`
	SkipInterfaces []string `yaml:"skip_interfaces" json:"-"`
	PingInterval   int      `yaml:"ping_interval" json:"ping_interval"`
	Online         bool     `yaml:"-" json:"online"`
	LastPinged     string   `yaml:"-" json:"last_pinged"`
}

type HostState struct {
	Online     bool
	LastPinged string
}

var hostStates = struct {
	sync.RWMutex
	Status map[string]HostState
}{Status: make(map[string]HostState)}

const (
	maxRequestBodyBytes   int64 = 64 * 1024
	maxEmailBytes               = 254
	minimumPasswordLength       = 12
	maxPasswordBytes            = 72
	maxDeviceIDLength           = 64
)

func validateEmail(email string) (string, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return "", errors.New("email is required")
	}
	if len(email) > maxEmailBytes {
		return "", fmt.Errorf("email must be at most %d bytes", maxEmailBytes)
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email {
		return "", errors.New("email must be a valid address")
	}
	return email, nil
}

func validatePassword(password string, requireMinimum bool) error {
	if password == "" {
		return errors.New("password is required")
	}
	if !utf8.ValidString(password) {
		return errors.New("password must be valid UTF-8")
	}
	if requireMinimum && utf8.RuneCountInString(password) < minimumPasswordLength {
		return fmt.Errorf("password must be at least %d characters", minimumPasswordLength)
	}
	if len(password) > maxPasswordBytes {
		return fmt.Errorf("password must be at most %d bytes", maxPasswordBytes)
	}
	return nil
}

func validateDeviceID(id string) error {
	if id == "" {
		return errors.New("device ID is required")
	}
	if len(id) > maxDeviceIDLength {
		return fmt.Errorf("device ID must be at most %d characters", maxDeviceIDLength)
	}
	for index := 0; index < len(id); index++ {
		character := id[index]
		isLetterOrDigit := character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9'
		if (index == 0 && !isLetterOrDigit) ||
			(index > 0 && !isLetterOrDigit && character != '.' && character != '_' && character != '-') {
			return errors.New("device ID must start with a letter or number and contain only letters, numbers, '.', '_' or '-'")
		}
	}
	return nil
}

func configuredDeviceIDs() (map[string]struct{}, error) {
	hosts, err := LoadHosts()
	if err != nil {
		return nil, err
	}
	configured := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		configured[host.ID] = struct{}{}
	}
	return configured, nil
}

func validateDeviceIDs(deviceIDs []string, configured map[string]struct{}) error {
	seen := make(map[string]struct{}, len(deviceIDs))
	for _, deviceID := range deviceIDs {
		if err := validateDeviceID(deviceID); err != nil {
			return err
		}
		if _, duplicate := seen[deviceID]; duplicate {
			return fmt.Errorf("device ID %q must not be repeated", deviceID)
		}
		if _, exists := configured[deviceID]; !exists {
			return fmt.Errorf("device ID %q is not configured", deviceID)
		}
		seen[deviceID] = struct{}{}
	}
	return nil
}

func findHost(id string) (*Host, error) {
	hosts, err := LoadHosts()
	if err != nil {
		return nil, err
	}
	for i := range hosts {
		if hosts[i].ID == id {
			return &hosts[i], nil
		}
	}
	return nil, fmt.Errorf("host %s not found", id)
}

// ----------------- API Handlers -----------------

type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func handleLogin(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	email, err := validateEmail(req.Email)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validatePassword(req.Password, false); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Email = email

	clientIP := c.ClientIP()
	if retryAfter := loginLimiter.retryAfter(clientIP, req.Email, time.Now()); retryAfter > 0 {
		loginLimiter.recordRateLimited()
		c.Header("Retry-After", strconv.Itoa(int(math.Ceil(retryAfter.Seconds()))))
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many login attempts; try again later"})
		return
	}

	user, err := GetUserByEmail(req.Email)
	if err != nil {
		// If no administrator exists, atomically create this user as the first one.
		hasAdmins, dbErr := HasAdmins()
		if dbErr == nil && !hasAdmins {
			if err := validatePassword(req.Password, true); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			hash, hashErr := HashPassword(req.Password)
			if hashErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
				return
			}
			err = CreateInitialAdmin(req.Email, hash)
			if err != nil {
				if errors.Is(err, ErrInitialAdminExists) {
					loginLimiter.recordFailure(clientIP, req.Email, time.Now())
					c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
				} else {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create initial admin user"})
				}
				return
			}
			// Fetch the newly created user
			user, err = GetUserByEmail(req.Email)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve new admin user"})
				return
			}
		} else {
			CheckPasswordHash(req.Password, dummyPasswordHash)
			loginLimiter.recordFailure(clientIP, req.Email, time.Now())
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
			return
		}
	} else if !CheckPasswordHash(req.Password, user.PasswordHash) {
		loginLimiter.recordFailure(clientIP, req.Email, time.Now())
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}
	loginLimiter.recordSuccess(clientIP, req.Email)

	token, err := GenerateJWT(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	setAuthCookie(c, token)
	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":       user.ID,
			"email":    user.Email,
			"is_admin": user.IsAdmin,
		},
	})
}

func handleLoginMetrics(c *gin.Context) {
	c.JSON(http.StatusOK, loginLimiter.metrics())
}

func handleLogout(c *gin.Context) {
	if err := RevokeUserTokens(c.GetUint("userID"), c.GetUint("tokenVersion")); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
		return
	}

	clearAuthCookie(c)
	c.Status(http.StatusNoContent)
}

func handleCurrentUser(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"id":       c.GetUint("userID"),
		"email":    c.GetString("userEmail"),
		"is_admin": c.GetBool("isAdmin"),
	})
}

// Check if user is authorized for a specific device based on UserDevice mapping
func isAuthorizedForDevice(userID uint, deviceID string, isAdmin bool) bool {
	if isAdmin {
		return true
	}
	user, err := GetUserByID(userID)
	if err != nil {
		return false
	}
	for _, dev := range user.Devices {
		if dev.DeviceID == deviceID {
			return true
		}
	}
	return false
}

func handleGetHosts(c *gin.Context) {
	hosts, err := LoadHosts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error loading hosts"})
		return
	}

	userID := c.GetUint("userID")
	isAdmin := c.GetBool("isAdmin")

	// Filter hosts based on authorization
	var authorizedHosts []Host
	for i := range hosts {
		if isAuthorizedForDevice(userID, hosts[i].ID, isAdmin) {
			// Attach online state to hosts from cache
			hostStates.RLock()
			state := hostStates.Status[hosts[i].ID]
			hosts[i].Online = state.Online
			if state.LastPinged == "" {
				hosts[i].LastPinged = "N/A"
			} else {
				hosts[i].LastPinged = state.LastPinged
			}
			hostStates.RUnlock()

			authorizedHosts = append(authorizedHosts, hosts[i])
		}
	}

	c.JSON(http.StatusOK, authorizedHosts)
}

func handleWOL(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetUint("userID")
	isAdmin := c.GetBool("isAdmin")

	if !isAuthorizedForDevice(userID, id, isAdmin) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized to access this device"})
		return
	}

	target, err := findHost(id)
	if err != nil {
		respondActionFailure(c, "wol target lookup", http.StatusNotFound, "Device not found")
		return
	}

	if err := SendWol(target); err != nil {
		respondActionFailure(c, "wake-on-LAN", http.StatusInternalServerError, "Unable to send Wake-on-LAN packet")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Magic Packet sent successfully to " + target.Name})
}

func handleShutdown(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetUint("userID")
	isAdmin := c.GetBool("isAdmin")

	if !isAuthorizedForDevice(userID, id, isAdmin) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized to access this device"})
		return
	}

	target, err := findHost(id)
	if err != nil {
		respondActionFailure(c, "shutdown target lookup", http.StatusNotFound, "Device not found")
		return
	}

	err = RemoteShutdown(target)
	if err != nil {
		respondActionFailure(c, "shutdown", http.StatusInternalServerError, "Unable to shut down device")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Shutdown command received from " + target.Name})
}

// ---- Admin API ----

func handleGetUsers(c *gin.Context) {
	var users []User
	// Preload the devices for the users so the admin can see them
	if err := DB.Preload("Devices").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
		return
	}

	// We don't want to return password hashes, so we clear them manually or map to a DTO
	// However, json:"-" on PasswordHash already hides it.
	c.JSON(http.StatusOK, users)
}

type CreateUserRequest struct {
	Email    string   `json:"email" binding:"required"`
	Password string   `json:"password" binding:"required"`
	IsAdmin  bool     `json:"is_admin"`
	Devices  []string `json:"devices"`
}

func handleCreateUser(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	email, err := validateEmail(req.Email)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validatePassword(req.Password, true); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.Devices) > 0 {
		configured, err := configuredDeviceIDs()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load configured devices"})
			return
		}
		if err := validateDeviceIDs(req.Devices, configured); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	err = CreateUser(email, hash, req.IsAdmin, req.Devices)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "User created successfully"})
}

type UpdateUserRequest struct {
	Password *string   `json:"password"` // optional
	IsAdmin  *bool     `json:"is_admin"` // optional
	Devices  *[]string `json:"devices"`  // optional; an empty list clears assignments
}

func handleUpdateUser(c *gin.Context) {
	id := c.Param("id")
	userID, err := strconv.Atoi(id)
	if err != nil || userID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Password != nil {
		if err := validatePassword(*req.Password, true); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	if req.Devices != nil && len(*req.Devices) > 0 {
		configured, err := configuredDeviceIDs()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load configured devices"})
			return
		}
		if err := validateDeviceIDs(*req.Devices, configured); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	var hashPtr *string
	if req.Password != nil {
		hash, err := HashPassword(*req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}
		hashPtr = &hash
	}

	err = UpdateUser(uint(userID), hashPtr, req.IsAdmin, req.Devices)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User updated successfully"})
}

func handleDeleteUser(c *gin.Context) {
	id := c.Param("id")
	userID, err := strconv.Atoi(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	if err := DeleteUser(uint(userID)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		if errors.Is(err, ErrLastAdministrator) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete the last administrator"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}

func handleCheckSetup(c *gin.Context) {
	hasAdmins, err := HasAdmins()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check setup status"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"needs_setup": !hasAdmins})
}

func securityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Security-Policy", "default-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; object-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'")
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Next()
	}
}

func requestBodyLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBodyBytes)
		}
		c.Next()
	}
}

const requestIDContextKey = "requestID"

func newRequestID() string {
	var value [16]byte
	if _, err := cryptoRand.Read(value[:]); err == nil {
		return hex.EncodeToString(value[:])
	}
	return strconv.FormatInt(time.Now().UnixNano(), 16)
}

func handleHealthz(c *gin.Context) {
	if _, err := loadHostsWithStatus(); err != nil || DB == nil {
		c.Status(http.StatusServiceUnavailable)
		return
	}

	sqlDB, err := DB.DB()
	if err != nil {
		c.Status(http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		c.Status(http.StatusServiceUnavailable)
		return
	}

	c.Status(http.StatusNoContent)
}

func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := newRequestID()
		c.Set(requestIDContextKey, requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

func requestIDFor(c *gin.Context) string {
	if requestID, exists := c.Get(requestIDContextKey); exists {
		if value, ok := requestID.(string); ok && value != "" {
			return value
		}
	}
	requestID := newRequestID()
	c.Set(requestIDContextKey, requestID)
	c.Header("X-Request-ID", requestID)
	return requestID
}

func respondActionFailure(c *gin.Context, action string, status int, message string) {
	requestID := requestIDFor(c)
	log.Printf("request_id=%s action=%s failed", requestID, action)
	c.JSON(status, gin.H{"error": message, "request_id": requestID})
}

func newRouter(trustedProxies []string) (*gin.Engine, error) {
	r := gin.New()

	if err := r.SetTrustedProxies(trustedProxies); err != nil {
		return nil, fmt.Errorf("configure trusted proxies: %w", err)
	}
	r.Use(requestIDMiddleware())
	r.Use(requestBodyLimitMiddleware())
	r.Use(securityHeadersMiddleware())
	r.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: []string{"/api/hosts"},
	}))
	r.Use(gin.Recovery())

	// Public Routes
	r.GET("/healthz", handleHealthz)
	r.POST("/api/login", handleLogin)
	r.GET("/api/setup", handleCheckSetup)

	// Protected Routes
	protected := r.Group("/api")
	protected.Use(AuthMiddleware())
	protected.POST("/logout", handleLogout)
	protected.GET("/session", handleCurrentUser)

	protected.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	protected.GET("/hosts", handleGetHosts)
	protected.POST("/wol/:id", handleWOL)
	protected.POST("/shutdown/:id", handleShutdown)

	// Admin Routes
	adminGroup := protected.Group("/users")
	adminGroup.Use(AdminMiddleware())
	adminGroup.GET("", handleGetUsers)
	adminGroup.POST("", handleCreateUser)
	adminGroup.PUT("/:id", handleUpdateUser)
	adminGroup.DELETE("/:id", handleDeleteUser)

	metricsGroup := protected.Group("/metrics")
	metricsGroup.Use(AdminMiddleware())
	metricsGroup.GET("/login", handleLoginMetrics)

	// Serve static files from the React frontend "dist" folder
	frontendPath := "/app/frontend/dist" // Default path for Docker
	if _, err := os.Stat("../frontend/dist/index.html"); err == nil {
		frontendPath = "../frontend/dist" // Path if running from backend folder
	} else if _, err := os.Stat("./frontend/dist/index.html"); err == nil {
		frontendPath = "./frontend/dist" // Path if running from project root
	}

	if _, err := os.Stat(frontendPath + "/index.html"); err == nil {
		r.Static("/assets", frontendPath+"/assets")
		r.StaticFile("/power.svg", frontendPath+"/power.svg")
		r.LoadHTMLGlob(frontendPath + "/index.html")

		// Catch-all route for React Router
		r.NoRoute(func(c *gin.Context) {
			c.HTML(http.StatusOK, "index.html", nil)
		})
	}

	return r, nil
}

func newHTTPServer(handler http.Handler, address string) *http.Server {
	return &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}

func main() {
	if err := initializeJWTSecret(); err != nil {
		log.Fatalf("Invalid JWT configuration: %v", err)
	}

	fmt.Println("Main Starting...")

	// Parse CLI flags
	addUserEmail := flag.String("adduser", "", "Email of the user to add")
	addUserPass := flag.String("password", "", "Password for the new user")
	addUserAdmin := flag.Bool("admin", false, "Make the new user an admin")
	addUserDevices := flag.String("devices", "", "Comma-separated list of allowed device IDs")
	flag.Parse()

	// Initialize Database
	InitDB()

	// Handle CLI user creation
	if *addUserEmail != "" {
		if *addUserPass == "" {
			log.Fatal("Password is required when adding a user")
		}
		hash, err := HashPassword(*addUserPass)
		if err != nil {
			log.Fatalf("Error hashing password: %v", err)
		}
		devices := []string{}
		if *addUserDevices != "" {
			devices = strings.Split(*addUserDevices, ",")
		}
		err = CreateUser(*addUserEmail, hash, *addUserAdmin, devices)
		if err != nil {
			log.Fatalf("Error creating user: %v", err)
		}
		fmt.Printf("User %s created successfully.\n", *addUserEmail)
		os.Exit(0)
	}

	trustedProxies, err := parseTrustedProxies(os.Getenv("TRUSTED_PROXIES"))
	if err != nil {
		log.Fatalf("Invalid TRUSTED_PROXIES configuration: %v", err)
	}
	r, err := newRouter(trustedProxies)
	if err != nil {
		log.Fatalf("Failed to configure HTTP router: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pingDone := make(chan struct{})
	go func() {
		defer close(pingDone)
		StartPingManager(ctx)
	}()

	// Get port from environment variable, default to 8080 if not set
	port := os.Getenv("PORT")
	if port == "" {
		port = "7500"
	}

	bindAddress := strings.TrimSpace(os.Getenv("BIND_ADDRESS"))
	if bindAddress == "" {
		bindAddress = "127.0.0.1"
	}

	server := newHTTPServer(r, net.JoinHostPort(bindAddress, port))
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- server.ListenAndServe() }()
	select {
	case err := <-serveErrors:
		stop()
		<-pingDone
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	case <-ctx.Done():
		stop()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := server.Shutdown(shutdownCtx)
		cancel()
		if err != nil {
			server.Close()
		}
		<-serveErrors
		<-pingDone
		if err != nil {
			log.Fatal(err)
		}
	}
}
