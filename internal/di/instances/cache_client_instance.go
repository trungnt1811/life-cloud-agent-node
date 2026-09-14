package instances

import (
	"context"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/redis/go-redis/v9"

	"github.com/lifenetwork-ai/go-backend-template/constants"
	"github.com/lifenetwork-ai/go-backend-template/internal/platform/logger"
	"github.com/lifenetwork-ai/go-backend-template/internal/runtimeconfig"
)

var (
	goCacheOnce     sync.Once
	goCacheInstance *cache.Cache

	redisOnce           sync.Once
	redisClientInstance *redis.Client
)

func GoCacheClientInstance() *cache.Cache {
	goCacheOnce.Do(func() {
		logger.GetLogger().Info("Initializing GoCache instance")
		goCacheInstance = cache.New(constants.DefaultExpiration, constants.CleanupInterval)
	})
	return goCacheInstance
}

func RedisClientInstance(config runtimeconfig.RedisConfig) *redis.Client {
	redisOnce.Do(func() {
		redisConf := runtimeconfig.NormalizeRedisConfig(config)
		logger.GetLogger().Info("Initializing Redis client", logger.String("address", redisConf.Address))

		redisClientInstance = redis.NewClient(redisOptionsFromConfig(redisConf))

		// Validate connection
		if err := redisClientInstance.Ping(context.Background()).Err(); err != nil {
			logger.GetLogger().Fatal("Failed to connect Redis", logger.Err(err))
		}
	})
	return redisClientInstance
}

func redisOptionsFromConfig(config runtimeconfig.RedisConfig) *redis.Options {
	config = runtimeconfig.NormalizeRedisConfig(config)
	return &redis.Options{
		Addr:         config.Address,
		Password:     config.Password,
		PoolSize:     config.PoolSize,
		MinIdleConns: config.MinIdleConns,
		DialTimeout:  time.Duration(config.DialTimeoutMs) * time.Millisecond,
		ReadTimeout:  time.Duration(config.ReadTimeoutMs) * time.Millisecond,
		WriteTimeout: time.Duration(config.WriteTimeoutMs) * time.Millisecond,
		PoolTimeout:  time.Duration(config.PoolTimeoutMs) * time.Millisecond,
	}
}
