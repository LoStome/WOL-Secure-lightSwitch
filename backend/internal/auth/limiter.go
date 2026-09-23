package auth

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	loginMaxFailures     = 5
	loginWindow          = 15 * time.Minute
	loginBackoff         = time.Minute
	LoginMaxTrackedKeys  = 10_000
	LoginCleanupInterval = time.Minute
)

// This hash belongs to no account and makes unknown-account logins pay the
// same bcrypt cost as wrong-password logins.
const DummyPasswordHash = "$2a$14$.v2MM6lQsbQtlFau9Yn2Je5bROrS04X2IHY1QukIOGYMuhbCkH9IG"

type loginAttempt struct {
	failures     int
	windowStart  time.Time
	blockedUntil time.Time
}

type LoginMetricsSnapshot struct {
	FailedAttemptsTotal      uint64 `json:"failed_attempts_total"`
	RateLimitedAttemptsTotal uint64 `json:"rate_limited_attempts_total"`
	TrackedKeys              int    `json:"tracked_keys"`
	EvictedKeysTotal         uint64 `json:"evicted_keys_total"`
}

type LoginAttemptLimiter struct {
	mu              sync.Mutex
	attempts        map[string]loginAttempt
	maxFailures     int
	window          time.Duration
	backoff         time.Duration
	maxTrackedKeys  int
	cleanupInterval time.Duration
	nextCleanup     time.Time

	failedAttemptsTotal      atomic.Uint64
	rateLimitedAttemptsTotal atomic.Uint64
	evictedKeysTotal         atomic.Uint64
}

func NewLoginAttemptLimiter(
	maxFailures int,
	window time.Duration,
	backoff time.Duration,
	maxTrackedKeys int,
	cleanupInterval time.Duration,
) *LoginAttemptLimiter {
	return &LoginAttemptLimiter{
		attempts:        make(map[string]loginAttempt),
		maxFailures:     maxFailures,
		window:          window,
		backoff:         backoff,
		maxTrackedKeys:  maxTrackedKeys,
		cleanupInterval: cleanupInterval,
	}
}

func loginAttemptKeys(ip, email string) [2]string {
	return [2]string{"ip:" + ip, "account:" + strings.ToLower(strings.TrimSpace(email))}
}

func (limiter *LoginAttemptLimiter) RetryAfter(ip, email string, now time.Time) time.Duration {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	limiter.cleanupIfDueLocked(now)
	var longest time.Duration
	for _, key := range loginAttemptKeys(ip, email) {
		attempt, exists := limiter.attempts[key]
		if !exists {
			continue
		}
		if limiter.attemptExpired(attempt, now) {
			delete(limiter.attempts, key)
			continue
		}
		if remaining := attempt.blockedUntil.Sub(now); remaining > longest {
			longest = remaining
		}
	}
	return longest
}

func (limiter *LoginAttemptLimiter) RecordFailure(ip, email string, now time.Time) {
	limiter.failedAttemptsTotal.Add(1)
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	limiter.cleanupIfDueLocked(now)
	for _, key := range loginAttemptKeys(ip, email) {
		attempt, exists := limiter.attempts[key]
		if !exists || limiter.attemptExpired(attempt, now) {
			if !exists {
				limiter.makeRoomLocked(now)
			}
			attempt = loginAttempt{windowStart: now}
		}
		attempt.failures++
		if attempt.failures >= limiter.maxFailures {
			attempt.blockedUntil = now.Add(limiter.backoff)
		}
		limiter.attempts[key] = attempt
	}
}

func (limiter *LoginAttemptLimiter) RecordRateLimited() {
	limiter.rateLimitedAttemptsTotal.Add(1)
}

func (limiter *LoginAttemptLimiter) RecordSuccess(ip, email string) {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	for _, key := range loginAttemptKeys(ip, email) {
		delete(limiter.attempts, key)
	}
}

func (limiter *LoginAttemptLimiter) Metrics() LoginMetricsSnapshot {
	limiter.mu.Lock()
	trackedKeys := len(limiter.attempts)
	limiter.mu.Unlock()

	return LoginMetricsSnapshot{
		FailedAttemptsTotal:      limiter.failedAttemptsTotal.Load(),
		RateLimitedAttemptsTotal: limiter.rateLimitedAttemptsTotal.Load(),
		TrackedKeys:              trackedKeys,
		EvictedKeysTotal:         limiter.evictedKeysTotal.Load(),
	}
}

func (limiter *LoginAttemptLimiter) attemptExpiresAt(attempt loginAttempt) time.Time {
	expiresAt := attempt.windowStart.Add(limiter.window)
	if attempt.blockedUntil.After(expiresAt) {
		return attempt.blockedUntil
	}
	return expiresAt
}

func (limiter *LoginAttemptLimiter) attemptExpired(attempt loginAttempt, now time.Time) bool {
	return !now.Before(limiter.attemptExpiresAt(attempt))
}

func (limiter *LoginAttemptLimiter) cleanupIfDueLocked(now time.Time) {
	if !limiter.nextCleanup.IsZero() && now.Before(limiter.nextCleanup) {
		return
	}
	limiter.cleanupExpiredLocked(now)
	limiter.nextCleanup = now.Add(limiter.cleanupInterval)
}

func (limiter *LoginAttemptLimiter) cleanupExpiredLocked(now time.Time) {
	for key, attempt := range limiter.attempts {
		if limiter.attemptExpired(attempt, now) {
			delete(limiter.attempts, key)
		}
	}
}

func (limiter *LoginAttemptLimiter) makeRoomLocked(now time.Time) {
	if len(limiter.attempts) < limiter.maxTrackedKeys {
		return
	}
	limiter.cleanupExpiredLocked(now)
	for len(limiter.attempts) >= limiter.maxTrackedKeys {
		var oldestKey string
		var oldestExpiry time.Time
		for key, attempt := range limiter.attempts {
			expiresAt := limiter.attemptExpiresAt(attempt)
			if oldestKey == "" || expiresAt.Before(oldestExpiry) {
				oldestKey = key
				oldestExpiry = expiresAt
			}
		}
		if oldestKey == "" {
			return
		}
		delete(limiter.attempts, oldestKey)
		limiter.evictedKeysTotal.Add(1)
	}
}

func ParseTrustedProxies(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}

	parts := strings.Split(value, ",")
	proxies := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, errors.New("TRUSTED_PROXIES contains an empty entry")
		}

		canonical := ""
		if strings.Contains(part, "/") {
			prefix, err := netip.ParsePrefix(part)
			if err != nil {
				return nil, fmt.Errorf("invalid trusted proxy %q: %w", part, err)
			}
			if prefix.Bits() == 0 {
				return nil, fmt.Errorf("trusted proxy %q permits every address", part)
			}
			canonical = prefix.Masked().String()
		} else {
			address, err := netip.ParseAddr(part)
			if err != nil || address.Zone() != "" {
				return nil, fmt.Errorf("invalid trusted proxy %q", part)
			}
			canonical = address.String()
		}

		if _, duplicate := seen[canonical]; duplicate {
			continue
		}
		seen[canonical] = struct{}{}
		proxies = append(proxies, canonical)
	}
	return proxies, nil
}

func NewDefaultLoginAttemptLimiter() *LoginAttemptLimiter {
	return NewLoginAttemptLimiter(loginMaxFailures, loginWindow, loginBackoff, LoginMaxTrackedKeys, LoginCleanupInterval)
}
