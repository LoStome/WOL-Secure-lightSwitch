package server

import (
	cryptoRand "crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

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
