package instances

import (
	"context"
	"strings"
	"sync"

	"github.com/lifenetwork-ai/life-cloud-agent-node/constants"
	"github.com/lifenetwork-ai/life-cloud-agent-node/infrastructures/caching"
	"github.com/lifenetwork-ai/life-cloud-agent-node/infrastructures/caching/types"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/runtimeconfig"
)

var (
	cacheOnce sync.Once
	cacheRepo types.CacheRepository
)

// CacheRepositoryInstance provides a singleton instance of CacheRepository.
func CacheRepositoryInstance(ctx context.Context, config runtimeconfig.CacheConfig) types.CacheRepository {
	cacheOnce.Do(func() {
		config = runtimeconfig.NormalizeCacheConfig(config)
		cacheType := strings.ToLower(strings.TrimSpace(config.Type))
		switch cacheType {
		case constants.CacheTypeRedis:
			// Using Redis cache
			logger.GetLogger().Info("Using Redis cache")
			cacheClient := caching.NewRedisCacheClient(RedisClientInstance(config.Redis), config.Redis.TTL)
			cacheRepo = caching.NewCachingRepository(ctx, config.AppName, cacheClient)
		case constants.CacheTypeInMemory:
			// Using in-memory GoCache implementation.
			logger.GetLogger().Info("Using in-memory cache (default)")
			cacheClient := caching.NewGoCacheClient(GoCacheClientInstance())
			cacheRepo = caching.NewCachingRepository(ctx, config.AppName, cacheClient)
		default:
			logger.GetLogger().Warn("Unsupported CACHE_TYPE; using in-memory cache", logger.String("cache_type", cacheType))
			cacheClient := caching.NewGoCacheClient(GoCacheClientInstance())
			cacheRepo = caching.NewCachingRepository(ctx, config.AppName, cacheClient)
		}
	})
	return cacheRepo
}
