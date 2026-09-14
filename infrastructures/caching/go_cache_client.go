package caching

import (
	"context"
	"errors"
	"reflect"
	"time"

	"github.com/patrickmn/go-cache"

	"github.com/lifenetwork-ai/go-backend-template/infrastructures/caching/types"
)

type goCacheClient struct {
	cache *cache.Cache
}

// NewGoCacheClient initializes a new cache client with default expiration and cleanup interval
func NewGoCacheClient(client *cache.Cache) types.CacheClient {
	return &goCacheClient{
		cache: client,
	}
}

// SetMany sets multiple items into the cache. Best-effort loop over items.
func (c *goCacheClient) SetMany(ctx context.Context, items []types.ClientBatchItem) error {
	for _, it := range items {
		c.cache.Set(it.Key, it.Value, it.Expiration)
	}
	return nil
}

func (c *goCacheClient) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	c.cache.Set(key, value, expiration)
	return nil
}

func (c *goCacheClient) Get(ctx context.Context, key string, dest any) error {
	cachedValue, found := c.cache.Get(key)
	if !found {
		return types.ErrCacheMiss
	}

	destVal := reflect.ValueOf(dest)
	if destVal.Kind() != reflect.Ptr || destVal.IsNil() {
		return types.ErrInvalidDestination
	}

	cachedVal := reflect.ValueOf(cachedValue)
	destType := destVal.Elem().Type()

	// Case 1: Direct assignment (same types)
	if cachedVal.Type().AssignableTo(destType) {
		destVal.Elem().Set(cachedVal)
		return nil
	}

	// Case 2: Cached is pointer, dest is value (*T -> T)
	if cachedVal.Kind() == reflect.Ptr && !cachedVal.IsNil() && cachedVal.Elem().Type().AssignableTo(destType) {
		destVal.Elem().Set(cachedVal.Elem())
		return nil
	}

	// Case 3: Cached is value, dest is pointer (T -> *T)
	if destType.Kind() == reflect.Ptr && cachedVal.Type().AssignableTo(destType.Elem()) {
		newPtr := reflect.New(destType.Elem())
		newPtr.Elem().Set(cachedVal)
		destVal.Elem().Set(newPtr)
		return nil
	}

	// Case 4: Both are pointers but different levels (*T -> **T or **T -> *T)
	if cachedVal.Kind() == reflect.Ptr && destType.Kind() == reflect.Ptr {
		if !cachedVal.IsNil() && cachedVal.Elem().Type().AssignableTo(destType.Elem()) {
			destVal.Elem().Set(cachedVal)
			return nil
		}
	}

	return types.ErrTypeMismatch
}

// Del deletes an item from the cache
func (c *goCacheClient) Del(ctx context.Context, key string) error {
	c.cache.Delete(key)
	return nil
}

func (c *goCacheClient) Incr(ctx context.Context, key string, delta int64) (int64, error) {
	if _, found := c.cache.Get(key); found {
		if err := c.cache.Increment(key, delta); err != nil {
			return 0, err
		}
		val, ok := c.cache.Get(key)
		if !ok {
			return 0, errors.New("key disappeared")
		}
		v64, ok := val.(int64)
		if !ok {
			return 0, types.ErrTypeMismatch
		}
		return v64, nil
	}

	// Create-on-miss for dev in-memory cache.
	if addErr := c.cache.Add(key, delta, cache.NoExpiration); addErr == nil {
		return delta, nil
	}

	// If another goroutine created it between Get and Add, retry the atomic increment.
	if err := c.cache.Increment(key, delta); err != nil {
		return 0, err
	}
	val, ok := c.cache.Get(key)
	if !ok {
		return 0, errors.New("key disappeared")
	}
	v64, ok := val.(int64)
	if !ok {
		return 0, types.ErrTypeMismatch
	}
	return v64, nil
}

func (c *goCacheClient) Decr(ctx context.Context, key string, delta int64) (int64, error) {
	if _, found := c.cache.Get(key); found {
		if err := c.cache.Decrement(key, delta); err != nil {
			return 0, err
		}
		val, ok := c.cache.Get(key)
		if !ok {
			return 0, errors.New("key disappeared")
		}
		v64, ok := val.(int64)
		if !ok {
			return 0, types.ErrTypeMismatch
		}
		return v64, nil
	}

	initial := -delta
	if addErr := c.cache.Add(key, initial, cache.NoExpiration); addErr == nil {
		return initial, nil
	}

	if err := c.cache.Decrement(key, delta); err != nil {
		return 0, err
	}
	val, ok := c.cache.Get(key)
	if !ok {
		return 0, errors.New("key disappeared")
	}
	v64, ok := val.(int64)
	if !ok {
		return 0, types.ErrTypeMismatch
	}
	return v64, nil
}

func (c *goCacheClient) TTL(ctx context.Context, key string) (time.Duration, error) {
	_, exp, found := c.cache.GetWithExpiration(key)
	if !found {
		return 0, types.ErrCacheMiss
	}
	// NoExpiration => exp is zero
	if exp.IsZero() {
		return -1, nil
	}
	d := time.Until(exp)
	if d < 0 {
		return 0, types.ErrCacheMiss
	}
	return d, nil
}

func (c *goCacheClient) ExpireNX(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	v, exp, found := c.cache.GetWithExpiration(key)
	if !found {
		return false, types.ErrCacheMiss
	}
	// Only set expiration if it currently has no expiration.
	if !exp.IsZero() {
		return false, nil
	}
	c.cache.Set(key, v, expiration)
	return true, nil
}
