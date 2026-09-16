package caching

import (
	"context"
	"encoding/json"
	"reflect"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/lifenetwork-ai/life-cloud-agent-node/infrastructures/caching/types"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
)

type redisCacheClient struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisCacheClient initializes a Redis cache client with a default TTL.
func NewRedisCacheClient(client *redis.Client, defaultTTL string) types.CacheClient {
	ttl, err := time.ParseDuration(defaultTTL)
	if err != nil {
		logger.GetLogger().Warn("Invalid REDIS_TTL format, using default", logger.String("redis_ttl", defaultTTL))
		ttl = 10 * time.Minute
	}

	return &redisCacheClient{
		client: client,
		ttl:    ttl,
	}
}

// SetMany stores multiple key-value pairs using a pipeline for efficiency.
// Each item follows the same TTL semantics as Set.
func (r *redisCacheClient) SetMany(ctx context.Context, items []types.ClientBatchItem) error {
	if len(items) == 0 {
		return nil
	}
	pipe := r.client.Pipeline()
	for _, it := range items {
		exp := it.Expiration
		if exp == 0 {
			exp = r.ttl
		} else if exp < 0 {
			exp = 0 // no expiration
		}
		data, err := json.Marshal(it.Value)
		if err != nil {
			logger.GetLogger().Error("Failed to marshal cache value", logger.String("key", it.Key), logger.Err(err))
			return err
		}
		pipe.Set(ctx, it.Key, data, exp)
	}
	_, err := pipe.Exec(ctx)
	if err != nil {
		logger.GetLogger().Error("Failed to exec cache pipeline", logger.Err(err))
		return err
	}
	return nil
}

// Get retrieves a value from Redis and unmarshals it into the provided destination.
// It handles various type assignments and returns appropriate errors.
func (r *redisCacheClient) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	if expiration == 0 {
		expiration = r.ttl
	} else if expiration < 0 {
		expiration = 0 // no expiration
	}

	data, err := json.Marshal(value)
	if err != nil {
		logger.GetLogger().Error("Failed to marshal cache value", logger.String("key", key), logger.Err(err))
		return err
	}

	if err := r.client.Set(ctx, key, data, expiration).Err(); err != nil {
		logger.GetLogger().Error("Failed to set cache", logger.String("key", key), logger.Err(err))
		return err
	}
	return nil
}

func (r *redisCacheClient) Get(ctx context.Context, key string, dest any) error {
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return types.ErrCacheMiss
		}
		logger.GetLogger().Error("Failed to get cache", logger.String("key", key), logger.Err(err))
		return err
	}

	destVal := reflect.ValueOf(dest)
	if destVal.Kind() != reflect.Ptr || destVal.IsNil() {
		return types.ErrInvalidDestination
	}

	dst := destVal.Elem() // level-1 deref

	switch dst.Kind() {
	case reflect.Ptr:
		// Case 3 & 4: dest is *T or **T
		newInner := reflect.New(dst.Type().Elem()) // *T
		if err := json.Unmarshal([]byte(val), newInner.Interface()); err != nil {
			// fallback re-decode if JSON type is generic map/array
			var tmp any
			if e2 := json.Unmarshal([]byte(val), &tmp); e2 != nil {
				logger.GetLogger().Error("Failed to unmarshal cache pointer value", logger.String("key", key), logger.Err(err))
				return types.ErrTypeMismatch
			}
			b, _ := json.Marshal(tmp)
			if e3 := json.Unmarshal(b, newInner.Interface()); e3 != nil {
				return types.ErrTypeMismatch
			}
		}
		dst.Set(newInner)
		return nil
	default:
		// Case 1 & 2: dest is T => unmarshal directly into &T
		if err := json.Unmarshal([]byte(val), dest); err != nil {
			// fallback re-decode if JSON type is generic map/array
			var tmp any
			if e2 := json.Unmarshal([]byte(val), &tmp); e2 != nil {
				logger.GetLogger().Error("Failed to unmarshal cache value", logger.String("key", key), logger.Err(err))
				return types.ErrTypeMismatch
			}
			b, _ := json.Marshal(tmp)
			if e3 := json.Unmarshal(b, dest); e3 != nil {
				return types.ErrTypeMismatch
			}
		}
		return nil
	}
}

// Del removes a cached key from Redis.
// It logs an error if the deletion fails.
func (r *redisCacheClient) Del(ctx context.Context, key string) error {
	if err := r.client.Del(ctx, key).Err(); err != nil {
		logger.GetLogger().Error("Failed to delete cache", logger.String("key", key), logger.Err(err))
		return err
	}
	return nil
}

func (r *redisCacheClient) Incr(ctx context.Context, key string, delta int64) (int64, error) {
	return r.client.IncrBy(ctx, key, delta).Result()
}

func (r *redisCacheClient) Decr(ctx context.Context, key string, delta int64) (int64, error) {
	return r.client.DecrBy(ctx, key, delta).Result()
}

func (r *redisCacheClient) TTL(ctx context.Context, key string) (time.Duration, error) {
	ttl, err := r.client.TTL(ctx, key).Result()
	if err != nil {
		return 0, err
	}

	// Redis semantics:
	// -2 => key does not exist
	// -1 => key exists but has no associated expiration
	// >=0 => TTL in seconds (duration)
	if ttl == -2*time.Second {
		return 0, types.ErrCacheMiss
	}
	// Normalize Redis "-1s" (no expiration) to time.Duration(-1) to match go-cache client.
	if ttl == -1*time.Second {
		return time.Duration(-1), nil
	}
	return ttl, nil
}

func (r *redisCacheClient) ExpireNX(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	ok, err := r.client.ExpireNX(ctx, key, expiration).Result()
	if err != nil {
		return false, err
	}
	if ok {
		return true, nil
	}

	// EXPIRENX returns false for both:
	// - missing key
	// - key exists but already has an expiration
	//
	// For portability with go-cache client, treat missing key as ErrCacheMiss.
	ttl, err := r.client.TTL(ctx, key).Result()
	if err != nil {
		return false, err
	}
	if ttl == -2*time.Second {
		return false, types.ErrCacheMiss
	}

	return false, nil
}
