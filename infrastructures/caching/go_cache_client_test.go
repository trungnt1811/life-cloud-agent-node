package caching

import (
	"context"
	"testing"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/life-cloud-agent-node/infrastructures/caching/types"
)

type testStruct struct {
	ID   int
	Name string
}

func TestGoCacheClient_Get_CacheMiss(t *testing.T) {
	client := NewGoCacheClient(cache.New(5*time.Minute, 10*time.Minute))

	var got int
	err := client.Get(context.Background(), "missing", &got)
	require.ErrorIs(t, err, types.ErrCacheMiss)
}

func TestGoCacheClient_Get_InvalidDestination(t *testing.T) {
	client := NewGoCacheClient(cache.New(5*time.Minute, 10*time.Minute))
	require.NoError(t, client.Set(context.Background(), "k", 123, time.Minute))

	err := client.Get(context.Background(), "k", 0)
	require.ErrorIs(t, err, types.ErrInvalidDestination)

	err = client.Get(context.Background(), "k", (*int)(nil))
	require.ErrorIs(t, err, types.ErrInvalidDestination)
}

func TestGoCacheClient_Get_DirectAssignment(t *testing.T) {
	client := NewGoCacheClient(cache.New(5*time.Minute, 10*time.Minute))
	require.NoError(t, client.Set(context.Background(), "k", 123, time.Minute))

	var got int
	require.NoError(t, client.Get(context.Background(), "k", &got))
	require.Equal(t, 123, got)
}

func TestGoCacheClient_Get_CachedPointerToValue(t *testing.T) {
	client := NewGoCacheClient(cache.New(5*time.Minute, 10*time.Minute))
	value := &testStruct{ID: 1, Name: "a"}
	require.NoError(t, client.Set(context.Background(), "k", value, time.Minute))

	var got testStruct
	require.NoError(t, client.Get(context.Background(), "k", &got))
	require.Equal(t, *value, got)
}

func TestGoCacheClient_Get_CachedValueToPointerDest(t *testing.T) {
	client := NewGoCacheClient(cache.New(5*time.Minute, 10*time.Minute))
	value := testStruct{ID: 2, Name: "b"}
	require.NoError(t, client.Set(context.Background(), "k", value, time.Minute))

	var got *testStruct
	require.NoError(t, client.Get(context.Background(), "k", &got))
	require.NotNil(t, got)
	require.Equal(t, value, *got)
}

func TestGoCacheClient_Get_PointerToPointerDest(t *testing.T) {
	client := NewGoCacheClient(cache.New(5*time.Minute, 10*time.Minute))
	value := &testStruct{ID: 3, Name: "c"}
	require.NoError(t, client.Set(context.Background(), "k", value, time.Minute))

	var got *testStruct
	require.NoError(t, client.Get(context.Background(), "k", &got))
	require.NotNil(t, got)
	require.Equal(t, value, got)
}

func TestGoCacheClient_SetMany_EmptyAndNonEmpty(t *testing.T) {
	client := NewGoCacheClient(cache.New(5*time.Minute, 10*time.Minute))
	require.NoError(t, client.SetMany(context.Background(), nil))
	require.NoError(t, client.SetMany(context.Background(), []types.ClientBatchItem{}))

	require.NoError(t, client.SetMany(context.Background(), []types.ClientBatchItem{
		{Key: "k1", Value: 1, Expiration: time.Minute},
		{Key: "k2", Value: "v2", Expiration: time.Minute},
	}))

	var got1 int
	require.NoError(t, client.Get(context.Background(), "k1", &got1))
	require.Equal(t, 1, got1)

	var got2 string
	require.NoError(t, client.Get(context.Background(), "k2", &got2))
	require.Equal(t, "v2", got2)
}

func TestGoCacheClient_Get_TypeMismatch(t *testing.T) {
	client := NewGoCacheClient(cache.New(5*time.Minute, 10*time.Minute))
	require.NoError(t, client.Set(context.Background(), "k", 123, time.Minute))

	var got string
	err := client.Get(context.Background(), "k", &got)
	require.ErrorIs(t, err, types.ErrTypeMismatch)
}

func TestGoCacheClient_IncrDecr(t *testing.T) {
	client := NewGoCacheClient(cache.New(cache.NoExpiration, 10*time.Minute))

	val, err := client.Incr(context.Background(), "missing", 1)
	require.NoError(t, err)
	require.Equal(t, int64(1), val)

	require.NoError(t, client.Set(context.Background(), "k", int64(10), cache.NoExpiration))
	val, err = client.Incr(context.Background(), "k", 2)
	require.NoError(t, err)
	require.Equal(t, int64(12), val)

	val, err = client.Decr(context.Background(), "k", 5)
	require.NoError(t, err)
	require.Equal(t, int64(7), val)
}

func TestGoCacheClient_IncrDecr_WrongType(t *testing.T) {
	client := NewGoCacheClient(cache.New(cache.NoExpiration, 10*time.Minute))
	require.NoError(t, client.Set(context.Background(), "k", "not-int64", cache.NoExpiration))

	_, err := client.Incr(context.Background(), "k", 1)
	require.Error(t, err)

	_, err = client.Decr(context.Background(), "k", 1)
	require.Error(t, err)
}

func TestGoCacheClient_TTL_CacheMiss(t *testing.T) {
	client := NewGoCacheClient(cache.New(5*time.Minute, 10*time.Minute))
	_, err := client.TTL(context.Background(), "missing")
	require.ErrorIs(t, err, types.ErrCacheMiss)
}

func TestGoCacheClient_TTL_NoExpiration(t *testing.T) {
	client := NewGoCacheClient(cache.New(cache.NoExpiration, 10*time.Minute))
	require.NoError(t, client.Set(context.Background(), "k", "v", cache.NoExpiration))

	ttl, err := client.TTL(context.Background(), "k")
	require.NoError(t, err)
	require.Equal(t, time.Duration(-1), ttl)
}

func TestGoCacheClient_TTL_WithExpiration(t *testing.T) {
	client := NewGoCacheClient(cache.New(cache.NoExpiration, 10*time.Minute))
	require.NoError(t, client.Set(context.Background(), "k", "v", 200*time.Millisecond))

	ttl, err := client.TTL(context.Background(), "k")
	require.NoError(t, err)
	require.Greater(t, ttl, time.Duration(0))
	require.LessOrEqual(t, ttl, 200*time.Millisecond)
}

func TestGoCacheClient_ExpireNX_Missing(t *testing.T) {
	client := NewGoCacheClient(cache.New(cache.NoExpiration, 10*time.Minute))
	_, err := client.ExpireNX(context.Background(), "missing", time.Second)
	require.ErrorIs(t, err, types.ErrCacheMiss)
}

func TestGoCacheClient_ExpireNX_SetsOnlyOnce(t *testing.T) {
	client := NewGoCacheClient(cache.New(cache.NoExpiration, 10*time.Minute))
	require.NoError(t, client.Set(context.Background(), "k", "v", cache.NoExpiration))

	ok, err := client.ExpireNX(context.Background(), "k", 200*time.Millisecond)
	require.NoError(t, err)
	require.True(t, ok)

	ttl, err := client.TTL(context.Background(), "k")
	require.NoError(t, err)
	require.Greater(t, ttl, time.Duration(0))
	require.LessOrEqual(t, ttl, 200*time.Millisecond)

	// Second call should not overwrite existing expiration.
	ok, err = client.ExpireNX(context.Background(), "k", 2*time.Second)
	require.NoError(t, err)
	require.False(t, ok)
}

// TestRedisCacheClient_SetMany_EmptyItems tests empty items handling
func TestRedisCacheClient_SetMany_EmptyItems(t *testing.T) {
	// Create a redisCacheClient with nil client to test early return
	client := &redisCacheClient{
		client: nil,
		ttl:    10 * time.Minute,
	}

	// Empty items should return nil without calling redis
	err := client.SetMany(context.Background(), nil)
	require.NoError(t, err)

	err = client.SetMany(context.Background(), []types.ClientBatchItem{})
	require.NoError(t, err)
}

// Additional go_cache_client tests for better coverage

func TestGoCacheClient_Del_Success(t *testing.T) {
	underlying := cache.New(5*time.Minute, 10*time.Minute)
	client := NewGoCacheClient(underlying)

	// Set a value
	require.NoError(t, client.Set(context.Background(), "k", "v", time.Minute))

	// Verify it exists
	var got string
	require.NoError(t, client.Get(context.Background(), "k", &got))
	require.Equal(t, "v", got)

	// Delete it
	require.NoError(t, client.Del(context.Background(), "k"))

	// Verify it's gone
	err := client.Get(context.Background(), "k", &got)
	require.ErrorIs(t, err, types.ErrCacheMiss)
}

func TestGoCacheClient_Decr_CreateOnMiss(t *testing.T) {
	underlying := cache.New(cache.NoExpiration, 10*time.Minute)
	client := NewGoCacheClient(underlying)

	// Decr on missing key should create with -delta
	val, err := client.Decr(context.Background(), "missing", 5)
	require.NoError(t, err)
	require.Equal(t, int64(-5), val)
}

func TestGoCacheClient_Incr_AfterRace(t *testing.T) {
	underlying := cache.New(cache.NoExpiration, 10*time.Minute)
	client := NewGoCacheClient(underlying)

	// Pre-set a value
	underlying.Set("k", int64(100), cache.NoExpiration)

	// Incr should work on existing value
	val, err := client.Incr(context.Background(), "k", 10)
	require.NoError(t, err)
	require.Equal(t, int64(110), val)
}

func TestGoCacheClient_Decr_AfterRace(t *testing.T) {
	underlying := cache.New(cache.NoExpiration, 10*time.Minute)
	client := NewGoCacheClient(underlying)

	// Pre-set a value
	underlying.Set("k", int64(100), cache.NoExpiration)

	// Decr should work on existing value
	val, err := client.Decr(context.Background(), "k", 10)
	require.NoError(t, err)
	require.Equal(t, int64(90), val)
}

func TestGoCacheClient_TTL_Expired(t *testing.T) {
	underlying := cache.New(cache.NoExpiration, 10*time.Minute)
	client := NewGoCacheClient(underlying)

	// Set with very short TTL
	require.NoError(t, client.Set(context.Background(), "k", "v", 1*time.Millisecond))

	// Wait for expiration
	time.Sleep(5 * time.Millisecond)

	// TTL should return cache miss for expired items
	_, err := client.TTL(context.Background(), "k")
	require.ErrorIs(t, err, types.ErrCacheMiss)
}

func TestGoCacheClient_Get_StructTypes(t *testing.T) {
	type nested struct {
		Value int
	}
	type complexType struct {
		ID     int
		Name   string
		Nested nested
	}

	underlying := cache.New(5*time.Minute, 10*time.Minute)
	client := NewGoCacheClient(underlying)

	original := complexType{ID: 1, Name: "test", Nested: nested{Value: 42}}
	require.NoError(t, client.Set(context.Background(), "k", original, time.Minute))

	var got complexType
	require.NoError(t, client.Get(context.Background(), "k", &got))
	require.Equal(t, original, got)
}

func TestGoCacheClient_Get_SliceTypes(t *testing.T) {
	underlying := cache.New(5*time.Minute, 10*time.Minute)
	client := NewGoCacheClient(underlying)

	original := []int{1, 2, 3, 4, 5}
	require.NoError(t, client.Set(context.Background(), "k", original, time.Minute))

	var got []int
	require.NoError(t, client.Get(context.Background(), "k", &got))
	require.Equal(t, original, got)
}

func TestGoCacheClient_Get_MapTypes(t *testing.T) {
	underlying := cache.New(5*time.Minute, 10*time.Minute)
	client := NewGoCacheClient(underlying)

	original := map[string]int{"a": 1, "b": 2}
	require.NoError(t, client.Set(context.Background(), "k", original, time.Minute))

	var got map[string]int
	require.NoError(t, client.Get(context.Background(), "k", &got))
	require.Equal(t, original, got)
}
