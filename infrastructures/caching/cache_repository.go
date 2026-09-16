package caching

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lifenetwork-ai/life-cloud-agent-node/infrastructures/caching/types"
)

type cachingRepository struct {
	ctx     context.Context
	appName string
	client  types.CacheClient
}

// NewCachingRepository initializes a new caching repository
func NewCachingRepository(ctx context.Context, appName string, client types.CacheClient) types.CacheRepository {
	appName = strings.TrimSpace(appName)
	if appName == "" {
		appName = "life-cloud-agent-node"
	}
	return &cachingRepository{
		ctx:     ctx,
		appName: appName,
		client:  client,
	}
}

// prependAppPrefix ensures all cache keys have a consistent prefix
func (repo *cachingRepository) prependAppPrefix(key string) string {
	return fmt.Sprintf("%s_%s", repo.appName, key)
}

// SaveItem saves an item to the cache with a specified expiration time
func (repo *cachingRepository) SaveItem(key fmt.Stringer, val any, expire time.Duration) error {
	prefixedKey := repo.prependAppPrefix(key.String())
	return repo.client.Set(repo.ctx, prefixedKey, val, expire)
}

// SaveItems saves multiple items to the cache in a single batch where supported.
func (repo *cachingRepository) SaveItems(items []types.RepoBatchItem) error {
	if len(items) == 0 {
		return nil
	}
	clientItems := make([]types.ClientBatchItem, 0, len(items))
	for _, it := range items {
		clientItems = append(clientItems, types.ClientBatchItem{
			Key:        repo.prependAppPrefix(it.Key.String()),
			Value:      it.Value,
			Expiration: it.Expiration,
		})
	}
	return repo.client.SetMany(repo.ctx, clientItems)
}

// RetrieveItem retrieves an item from the cache
func (repo *cachingRepository) RetrieveItem(key fmt.Stringer, val any) error {
	prefixedKey := repo.prependAppPrefix(key.String())
	return repo.client.Get(repo.ctx, prefixedKey, val)
}

// RemoveItem removes an item from the cache
func (repo *cachingRepository) RemoveItem(key fmt.Stringer) error {
	prefixedKey := repo.prependAppPrefix(key.String())
	return repo.client.Del(repo.ctx, prefixedKey)
}

// IncrementItem increments an item in the cache
func (repo *cachingRepository) IncrementItem(key fmt.Stringer, delta int64) (int64, error) {
	prefixedKey := repo.prependAppPrefix(key.String())
	return repo.client.Incr(repo.ctx, prefixedKey, delta)
}

// DecrementItem decrements an item in the cache
func (repo *cachingRepository) DecrementItem(key fmt.Stringer, delta int64) (int64, error) {
	prefixedKey := repo.prependAppPrefix(key.String())
	return repo.client.Decr(repo.ctx, prefixedKey, delta)
}

// TTL gets the time-to-live of a cache item
func (repo *cachingRepository) TTL(key fmt.Stringer) (time.Duration, error) {
	prefixedKey := repo.prependAppPrefix(key.String())
	return repo.client.TTL(repo.ctx, prefixedKey)
}

// ExpireNX sets the expiration of a cache item if it does not already have one
func (repo *cachingRepository) ExpireNX(key fmt.Stringer, expire time.Duration) (bool, error) {
	prefixedKey := repo.prependAppPrefix(key.String())
	return repo.client.ExpireNX(repo.ctx, prefixedKey, expire)
}
