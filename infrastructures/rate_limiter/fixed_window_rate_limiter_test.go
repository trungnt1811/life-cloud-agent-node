package rate_limiter

import (
	"context"
	"testing"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/go-backend-template/infrastructures/caching"
)

func TestFixedWindowRateLimiter_AllowCountsAndReset(t *testing.T) {
	underlying := cache.New(cache.NoExpiration, 10*time.Minute)
	cacheClient := caching.NewGoCacheClient(underlying)
	cacheRepo := caching.NewCachingRepository(context.Background(), "go-backend-template-test", cacheClient)

	limiter := NewFixedWindowRateLimiter(cacheRepo)
	key := "user:1"
	window := 30 * time.Second

	allowed, count, ttl, err := limiter.Allow(key, 1, window)
	require.NoError(t, err)
	require.True(t, allowed)
	require.Equal(t, 1, count)
	require.Greater(t, ttl, time.Duration(0))
	require.LessOrEqual(t, ttl, window)

	allowed, count, ttl, err = limiter.Allow(key, 1, window)
	require.NoError(t, err)
	require.False(t, allowed)
	require.Equal(t, 2, count)
	require.Greater(t, ttl, time.Duration(0))
	require.LessOrEqual(t, ttl, window)

	require.NoError(t, limiter.ResetAttempts(key))

	allowed, count, ttl, err = limiter.Allow(key, 1, window)
	require.NoError(t, err)
	require.True(t, allowed)
	require.Equal(t, 1, count)
	require.Greater(t, ttl, time.Duration(0))
}

func TestFixedWindowRateLimiter_LimitZeroFirstCallDenied(t *testing.T) {
	underlying := cache.New(cache.NoExpiration, 10*time.Minute)
	cacheClient := caching.NewGoCacheClient(underlying)
	cacheRepo := caching.NewCachingRepository(context.Background(), "go-backend-template-test", cacheClient)

	limiter := NewFixedWindowRateLimiter(cacheRepo)
	key := "user:zero"

	allowed, count, ttl, err := limiter.Allow(key, 0, time.Minute)
	require.NoError(t, err)
	require.False(t, allowed)
	require.Equal(t, 1, count)
	require.Greater(t, ttl, time.Duration(0))
}

// Additional tests for better coverage

func TestFixedWindowRateLimiter_MultipleRequestsWithinLimit(t *testing.T) {
	underlying := cache.New(cache.NoExpiration, 10*time.Minute)
	cacheClient := caching.NewGoCacheClient(underlying)
	cacheRepo := caching.NewCachingRepository(context.Background(), "go-backend-template-test", cacheClient)

	limiter := NewFixedWindowRateLimiter(cacheRepo)
	key := "user:multi"
	window := 30 * time.Second
	limit := 5

	// All 5 requests within limit should be allowed
	for i := 1; i <= limit; i++ {
		allowed, count, _, err := limiter.Allow(key, limit, window)
		require.NoError(t, err)
		require.True(t, allowed, "Request %d should be allowed", i)
		require.Equal(t, i, count)
	}

	// 6th request should be denied
	allowed, count, _, err := limiter.Allow(key, limit, window)
	require.NoError(t, err)
	require.False(t, allowed)
	require.Equal(t, 6, count)
}

func TestFixedWindowRateLimiter_DifferentKeysIndependent(t *testing.T) {
	underlying := cache.New(cache.NoExpiration, 10*time.Minute)
	cacheClient := caching.NewGoCacheClient(underlying)
	cacheRepo := caching.NewCachingRepository(context.Background(), "go-backend-template-test", cacheClient)

	limiter := NewFixedWindowRateLimiter(cacheRepo)
	window := 30 * time.Second
	limit := 2

	// First user uses limit
	allowed, _, _, _ := limiter.Allow("user:a", limit, window)
	require.True(t, allowed)
	allowed, _, _, _ = limiter.Allow("user:a", limit, window)
	require.True(t, allowed)
	allowed, _, _, _ = limiter.Allow("user:a", limit, window)
	require.False(t, allowed)

	// Second user should still have full limit
	allowed, count, _, _ := limiter.Allow("user:b", limit, window)
	require.True(t, allowed)
	require.Equal(t, 1, count)
}

func TestFixedWindowRateLimiter_WindowExpiration(t *testing.T) {
	// Use short expiration for test
	underlying := cache.New(1*time.Millisecond, 1*time.Millisecond)
	cacheClient := caching.NewGoCacheClient(underlying)
	cacheRepo := caching.NewCachingRepository(context.Background(), "go-backend-template-test", cacheClient)

	limiter := NewFixedWindowRateLimiter(cacheRepo)
	key := "user:expiring"
	window := 5 * time.Millisecond
	limit := 1

	// First request allowed
	allowed, _, _, err := limiter.Allow(key, limit, window)
	require.NoError(t, err)
	require.True(t, allowed)

	// Immediate second request denied
	allowed, _, _, err = limiter.Allow(key, limit, window)
	require.NoError(t, err)
	require.False(t, allowed)

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	// After window expires, should be allowed again
	allowed, count, _, err := limiter.Allow(key, limit, window)
	require.NoError(t, err)
	require.True(t, allowed)
	require.Equal(t, 1, count)
}

func TestFixedWindowRateLimiter_ResetNonExistentKey(t *testing.T) {
	underlying := cache.New(cache.NoExpiration, 10*time.Minute)
	cacheClient := caching.NewGoCacheClient(underlying)
	cacheRepo := caching.NewCachingRepository(context.Background(), "go-backend-template-test", cacheClient)

	limiter := NewFixedWindowRateLimiter(cacheRepo)

	// Reset non-existent key should not error
	err := limiter.ResetAttempts("non:existent:key")
	require.NoError(t, err)
}

func TestFixedWindowRateLimiter_HighLimit(t *testing.T) {
	underlying := cache.New(cache.NoExpiration, 10*time.Minute)
	cacheClient := caching.NewGoCacheClient(underlying)
	cacheRepo := caching.NewCachingRepository(context.Background(), "go-backend-template-test", cacheClient)

	limiter := NewFixedWindowRateLimiter(cacheRepo)
	key := "user:highlimit"
	window := 30 * time.Second
	limit := 1000

	// Single request with high limit should be allowed
	allowed, count, _, err := limiter.Allow(key, limit, window)
	require.NoError(t, err)
	require.True(t, allowed)
	require.Equal(t, 1, count)
}
