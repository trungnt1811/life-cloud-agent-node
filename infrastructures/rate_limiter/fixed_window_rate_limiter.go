package rate_limiter

import (
	"time"

	cachetypes "github.com/lifenetwork-ai/go-backend-template/infrastructures/caching/types"
	"github.com/lifenetwork-ai/go-backend-template/infrastructures/rate_limiter/types"
)

type fixedWindowRateLimiter struct {
	cacheRepo cachetypes.CacheRepository
}

func NewFixedWindowRateLimiter(cache cachetypes.CacheRepository) types.RateLimiter {
	return &fixedWindowRateLimiter{cacheRepo: cache}
}

func (r *fixedWindowRateLimiter) Allow(key string, limit int, window time.Duration) (bool, int, time.Duration, error) {
	cacheKey := &cachetypes.Keyer{Raw: key}

	// 1) Atomic increment when possible (Redis). For go-cache, works if the key exists.
	// If missing, we'll initialize below.
	newCount64, err := r.cacheRepo.IncrementItem(cacheKey, 1)
	if err != nil {
		// Treat as "missing" or "not initialized" -> initialize window with count=1
		if err2 := r.cacheRepo.SaveItem(cacheKey, int64(1), window); err2 != nil {
			return true, 0, 0, err2
		}

		// best-effort TTL read for headers
		ttl, _ := r.cacheRepo.TTL(cacheKey)
		if ttl <= 0 {
			ttl = window
		}
		allowed := 1 <= limit
		return allowed, 1, ttl, nil
	}

	// 2) Ensure TTL is set exactly once (Redis), best-effort
	_, _ = r.cacheRepo.ExpireNX(cacheKey, window)

	// 3) TTL for headers (best-effort)
	ttl, errTTL := r.cacheRepo.TTL(cacheKey)
	if errTTL != nil || ttl <= 0 {
		ttl = window
	}

	count := int(newCount64)
	allowed := count <= limit
	return allowed, count, ttl, nil
}

func (r *fixedWindowRateLimiter) ResetAttempts(key string) error {
	cacheKey := &cachetypes.Keyer{Raw: key}
	return r.cacheRepo.RemoveItem(cacheKey)
}
