package main

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	loginMaxFailures = 5
	loginWindow      = 15 * time.Minute
	loginBackoff     = time.Minute
)

// This hash belongs to no account and makes unknown-account logins pay the
// same bcrypt cost as wrong-password logins.
const dummyPasswordHash = "$2a$14$.v2MM6lQsbQtlFau9Yn2Je5bROrS04X2IHY1QukIOGYMuhbCkH9IG"

var (
	loginLimiter = newLoginAttemptLimiter(loginMaxFailures, loginWindow, loginBackoff)

	failedLoginAttemptsTotal      atomic.Uint64
	rateLimitedLoginAttemptsTotal atomic.Uint64
)

type loginAttempt struct {
	failures     int
	windowStart  time.Time
	blockedUntil time.Time
}

type loginAttemptLimiter struct {
	mu          sync.Mutex
	attempts    map[string]loginAttempt
	maxFailures int
	window      time.Duration
	backoff     time.Duration
}

func newLoginAttemptLimiter(maxFailures int, window, backoff time.Duration) *loginAttemptLimiter {
	return &loginAttemptLimiter{
		attempts:    make(map[string]loginAttempt),
		maxFailures: maxFailures,
		window:      window,
		backoff:     backoff,
	}
}

func loginAttemptKeys(ip, email string) [2]string {
	return [2]string{"ip:" + ip, "account:" + strings.ToLower(strings.TrimSpace(email))}
}

func (limiter *loginAttemptLimiter) retryAfter(ip, email string, now time.Time) time.Duration {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	var longest time.Duration
	for _, key := range loginAttemptKeys(ip, email) {
		attempt, exists := limiter.attempts[key]
		if !exists {
			continue
		}
		if !now.Before(attempt.blockedUntil) && now.Sub(attempt.windowStart) >= limiter.window {
			delete(limiter.attempts, key)
			continue
		}
		if remaining := attempt.blockedUntil.Sub(now); remaining > longest {
			longest = remaining
		}
	}
	return longest
}

func (limiter *loginAttemptLimiter) recordFailure(ip, email string, now time.Time) {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	for _, key := range loginAttemptKeys(ip, email) {
		attempt := limiter.attempts[key]
		if attempt.windowStart.IsZero() || (!now.Before(attempt.blockedUntil) && now.Sub(attempt.windowStart) >= limiter.window) {
			attempt = loginAttempt{windowStart: now}
		}
		attempt.failures++
		if attempt.failures >= limiter.maxFailures {
			attempt.blockedUntil = now.Add(limiter.backoff)
		}
		limiter.attempts[key] = attempt
	}
	failedLoginAttemptsTotal.Add(1)
}

func (limiter *loginAttemptLimiter) recordSuccess(ip, email string) {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	for _, key := range loginAttemptKeys(ip, email) {
		delete(limiter.attempts, key)
	}
}
