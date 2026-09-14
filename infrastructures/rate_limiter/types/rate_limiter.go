package types

import "time"

//go:generate mockgen -source=rate_limiter.go -package=mocks -destination=../../mocks/mock_rate_limiter.go
type RateLimiter interface {
	// Allow performs an atomic attempt registration and returns:
	// - allowed: request is allowed or rate-limited
	// - count: current attempts in this window (after increment)
	// - ttl: remaining time until window resets
	Allow(key string, limit int, window time.Duration) (allowed bool, count int, ttl time.Duration, err error)

	// ResetAttempts deletes the key (best effort).
	ResetAttempts(key string) error
}
