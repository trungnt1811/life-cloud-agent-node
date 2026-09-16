package caching

import (
	"context"
	"testing"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/life-cloud-agent-node/infrastructures/caching/types"
)

const testAppName = "life-cloud-agent-node-test"

func TestCachingRepository_Prefixing_SaveRetrieveRemove(t *testing.T) {
	underlying := cache.New(5*time.Minute, 10*time.Minute)
	client := NewGoCacheClient(underlying)
	repo := NewCachingRepository(context.Background(), testAppName, client)

	key := &types.Keyer{Raw: "abc"}
	prefixed := testAppName + "_" + key.String()

	require.NoError(t, repo.SaveItem(key, "v", time.Minute))
	_, ok := underlying.Get(prefixed)
	require.True(t, ok)

	var got string
	require.NoError(t, repo.RetrieveItem(key, &got))
	require.Equal(t, "v", got)

	require.NoError(t, repo.RemoveItem(key))
	_, ok = underlying.Get(prefixed)
	require.False(t, ok)
}

func TestCachingRepository_SaveItems_Batch(t *testing.T) {
	underlying := cache.New(5*time.Minute, 10*time.Minute)
	client := NewGoCacheClient(underlying)
	repo := NewCachingRepository(context.Background(), testAppName, client)

	require.NoError(t, repo.SaveItems(nil))
	require.NoError(t, repo.SaveItems([]types.RepoBatchItem{}))

	items := []types.RepoBatchItem{
		{Key: &types.Keyer{Raw: "k1"}, Value: 1, Expiration: time.Minute},
		{Key: &types.Keyer{Raw: "k2"}, Value: 2, Expiration: 2 * time.Minute},
	}
	require.NoError(t, repo.SaveItems(items))

	_, ok := underlying.Get(testAppName + "_k1")
	require.True(t, ok)
	_, ok = underlying.Get(testAppName + "_k2")
	require.True(t, ok)
}

func TestCachingRepository_IncrementDecrement_Prefixing(t *testing.T) {
	underlying := cache.New(cache.NoExpiration, 10*time.Minute)
	client := NewGoCacheClient(underlying)
	repo := NewCachingRepository(context.Background(), testAppName, client)

	key := &types.Keyer{Raw: "counter"}
	prefixed := testAppName + "_" + key.String()

	// Prime the underlying cache with the prefixed key, because go-cache Increment requires existing numeric value.
	underlying.Set(prefixed, int64(10), cache.NoExpiration)

	val, err := repo.IncrementItem(key, 5)
	require.NoError(t, err)
	require.Equal(t, int64(15), val)

	val, err = repo.DecrementItem(key, 3)
	require.NoError(t, err)
	require.Equal(t, int64(12), val)
}

func TestCachingRepository_TTL_And_ExpireNX_Prefixing(t *testing.T) {
	underlying := cache.New(cache.NoExpiration, 10*time.Minute)
	client := NewGoCacheClient(underlying)
	repo := NewCachingRepository(context.Background(), testAppName, client)

	key := &types.Keyer{Raw: "ttl"}
	prefixed := testAppName + "_" + key.String()

	// Prime underlying with no-expiration item (like Redis key without TTL)
	underlying.Set(prefixed, "v", cache.NoExpiration)

	ttl, err := repo.TTL(key)
	require.NoError(t, err)
	require.Equal(t, time.Duration(-1), ttl)

	ok, err := repo.ExpireNX(key, 200*time.Millisecond)
	require.NoError(t, err)
	require.True(t, ok)

	ttl, err = repo.TTL(key)
	require.NoError(t, err)
	require.Greater(t, ttl, time.Duration(0))
	require.LessOrEqual(t, ttl, 200*time.Millisecond)

	// Second ExpireNX should not override existing expiration.
	ok, err = repo.ExpireNX(key, 2*time.Second)
	require.NoError(t, err)
	require.False(t, ok)
}
