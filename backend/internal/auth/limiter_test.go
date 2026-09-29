package auth

import (
	"fmt"
	"testing"
	"time"
)

func newTestLoginLimiter(maxFailures, maxTrackedKeys int, window, backoff time.Duration) *LoginAttemptLimiter {
	return NewLoginAttemptLimiter(maxFailures, window, backoff, maxTrackedKeys, time.Second)
}

func TestLoginLimiterKeysByAccountAndIP(t *testing.T) {
	now := time.Date(2026, time.September, 22, 12, 0, 0, 0, time.UTC)

	t.Run("account across IP addresses", func(t *testing.T) {
		limiter := newTestLoginLimiter(2, 100, time.Minute, time.Minute)
		limiter.RecordFailure("192.0.2.1", "victim@example.com", now)
		limiter.RecordFailure("192.0.2.2", "victim@example.com", now)

		if got := limiter.RetryAfter("192.0.2.3", "victim@example.com", now); got != time.Minute {
			t.Fatalf("account retryAfter = %v, want %v", got, time.Minute)
		}
	})

	t.Run("IP across accounts", func(t *testing.T) {
		limiter := newTestLoginLimiter(2, 100, time.Minute, time.Minute)
		limiter.RecordFailure("192.0.2.1", "first@example.com", now)
		limiter.RecordFailure("192.0.2.1", "second@example.com", now)

		if got := limiter.RetryAfter("192.0.2.1", "third@example.com", now); got != time.Minute {
			t.Fatalf("IP retryAfter = %v, want %v", got, time.Minute)
		}
	})
}

func TestLoginLimiterResetAndExpiration(t *testing.T) {
	now := time.Date(2026, time.September, 22, 12, 0, 0, 0, time.UTC)

	t.Run("successful login clears keys", func(t *testing.T) {
		limiter := newTestLoginLimiter(1, 100, time.Minute, time.Minute)
		limiter.RecordFailure("192.0.2.1", "user@example.com", now)
		limiter.RecordSuccess("192.0.2.1", "user@example.com")

		if got := limiter.RetryAfter("192.0.2.1", "user@example.com", now); got != 0 {
			t.Fatalf("retryAfter after success = %v, want 0", got)
		}
		if got := limiter.Metrics().TrackedKeys; got != 0 {
			t.Fatalf("tracked keys after success = %d, want 0", got)
		}
	})

	t.Run("global cleanup removes expired keys", func(t *testing.T) {
		limiter := newTestLoginLimiter(1, 100, 10*time.Second, time.Second)
		limiter.RecordFailure("192.0.2.1", "user@example.com", now)

		limiter.RetryAfter("192.0.2.99", "other@example.com", now.Add(11*time.Second))
		if got := limiter.Metrics().TrackedKeys; got != 0 {
			t.Fatalf("tracked keys after expiration = %d, want 0", got)
		}
	})
}

func TestLoginLimiterBoundsTrackedKeys(t *testing.T) {
	now := time.Date(2026, time.September, 22, 12, 0, 0, 0, time.UTC)
	limiter := newTestLoginLimiter(5, LoginMaxTrackedKeys, time.Hour, time.Minute)

	for index := 0; index < 6_000; index++ {
		limiter.RecordFailure(
			fmt.Sprintf("client-%d", index),
			fmt.Sprintf("user-%d@example.com", index),
			now,
		)
	}

	metrics := limiter.Metrics()
	if metrics.TrackedKeys != LoginMaxTrackedKeys {
		t.Fatalf("tracked keys = %d, want %d", metrics.TrackedKeys, LoginMaxTrackedKeys)
	}
	if metrics.EvictedKeysTotal == 0 {
		t.Fatal("bounded limiter did not report any evictions")
	}
}

func TestLoginLimiterMetrics(t *testing.T) {
	now := time.Date(2026, time.September, 22, 12, 0, 0, 0, time.UTC)
	limiter := newTestLoginLimiter(1, 100, time.Minute, time.Minute)
	limiter.RecordFailure("192.0.2.1", "user@example.com", now)
	limiter.RecordRateLimited()

	metrics := limiter.Metrics()
	if metrics.FailedAttemptsTotal != 1 || metrics.RateLimitedAttemptsTotal != 1 {
		t.Fatalf("metrics = %+v, want one failure and one rate-limited attempt", metrics)
	}
	if metrics.TrackedKeys != 2 || metrics.EvictedKeysTotal != 0 {
		t.Fatalf("metrics = %+v, want two tracked keys and no evictions", metrics)
	}
}
