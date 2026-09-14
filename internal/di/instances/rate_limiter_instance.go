package instances

import (
	"context"
	"sync"

	cachetypes "github.com/lifenetwork-ai/go-backend-template/infrastructures/caching/types"
	ratelimiter "github.com/lifenetwork-ai/go-backend-template/infrastructures/rate_limiter"
	"github.com/lifenetwork-ai/go-backend-template/infrastructures/rate_limiter/types"
	"github.com/lifenetwork-ai/go-backend-template/internal/platform/logger"
	"github.com/lifenetwork-ai/go-backend-template/internal/runtimeconfig"
)

var (
	rateLimiterOnce sync.Once
	rateLimiterInst types.RateLimiter
)

// RateLimiterInstance provides a singleton RateLimiter (Fixed-Window) that
// uses the shared CacheRepository instance under the hood.
func RateLimiterInstance(ctx context.Context, cacheRepo cachetypes.CacheRepository) types.RateLimiter {
	rateLimiterOnce.Do(func() {
		logger.GetLogger().Info("Initializing Fixed-Window Rate Limiter")
		if cacheRepo == nil {
			cacheRepo = CacheRepositoryInstance(ctx, runtimeconfig.CacheConfig{})
		}
		rateLimiterInst = ratelimiter.NewFixedWindowRateLimiter(cacheRepo)
	})
	return rateLimiterInst
}
