package server

import (
	"fmt"
	"net/http"
	"os"

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
