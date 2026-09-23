package server

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

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
