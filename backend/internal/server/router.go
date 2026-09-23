package server

import (
	"context"
	cryptoRand "crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"secure-switch-backend/internal/auth"
	"secure-switch-backend/internal/config"
	"secure-switch-backend/internal/device"
	"secure-switch-backend/internal/store"
)

type App struct {
	Config   *config.Loader
	Store    *store.Store
	Auth     *auth.Service
	Limiter  *auth.LoginAttemptLimiter
	Monitor  *device.Monitor
	Wake     func(*config.Host) error
	Shutdown func(*config.Host) error
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

func RequestBodyLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxRequestBodyBytes)
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

func (a *App) handleHealthz(c *gin.Context) {
	if _, err := a.Config.LoadHostsWithStatus(); err != nil || a.Store == nil || a.Store.DB == nil {
		c.Status(http.StatusServiceUnavailable)
		return
	}

	sqlDB, err := a.Store.DB.DB()
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

func (a *App) Router(trustedProxies []string) (*gin.Engine, error) {
	r := gin.New()

	if err := r.SetTrustedProxies(trustedProxies); err != nil {
		return nil, fmt.Errorf("configure trusted proxies: %w", err)
	}
	r.Use(requestIDMiddleware())
	r.Use(RequestBodyLimitMiddleware())
	r.Use(securityHeadersMiddleware())
	r.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: []string{"/api/hosts"},
	}))
	r.Use(gin.Recovery())

	// Public Routes
	r.GET("/healthz", a.handleHealthz)
	r.POST("/api/login", a.HandleLogin)
	r.GET("/api/setup", a.handleCheckSetup)

	// Protected Routes
	protected := r.Group("/api")
	protected.Use(a.Auth.AuthMiddleware())
	protected.POST("/logout", a.HandleLogout)
	protected.GET("/session", a.handleCurrentUser)

	protected.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	protected.GET("/hosts", a.handleGetHosts)
	protected.POST("/wol/:id", a.handleWOL)
	protected.POST("/shutdown/:id", a.handleShutdown)

	// Admin Routes
	adminGroup := protected.Group("/users")
	adminGroup.Use(auth.AdminMiddleware())
	adminGroup.GET("", a.handleGetUsers)
	adminGroup.POST("", a.handleCreateUser)
	adminGroup.PUT("/:id", a.HandleUpdateUser)
	adminGroup.DELETE("/:id", a.handleDeleteUser)

	metricsGroup := protected.Group("/metrics")
	metricsGroup.Use(auth.AdminMiddleware())
	metricsGroup.GET("/login", a.handleLoginMetrics)

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

func NewHTTPServer(handler http.Handler, address string) *http.Server {
	return &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}
