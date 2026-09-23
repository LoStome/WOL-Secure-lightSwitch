package main

import (
	"time"

	"secure-switch-backend/internal/auth"
)

// Temporary compatibility adapters while handlers move into internal/server.
const loginMaxTrackedKeys = auth.LoginMaxTrackedKeys
const loginCleanupInterval = auth.LoginCleanupInterval
const dummyPasswordHash = auth.DummyPasswordHash

type loginAttemptLimiter = auth.LoginAttemptLimiter
type loginMetricsSnapshot = auth.LoginMetricsSnapshot

var loginLimiter = auth.NewDefaultLoginAttemptLimiter()

func newLoginAttemptLimiter(maxFailures int, window, backoff time.Duration, maxTrackedKeys int, cleanupInterval time.Duration) *loginAttemptLimiter {
	return auth.NewLoginAttemptLimiter(maxFailures, window, backoff, maxTrackedKeys, cleanupInterval)
}

func parseTrustedProxies(value string) ([]string, error) {
	return auth.ParseTrustedProxies(value)
}
