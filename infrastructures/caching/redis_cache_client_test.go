package caching

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/go-redis/redismock/v9"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/life-cloud-agent-node/infrastructures/caching/types"
)

const testValConstant = "test_val"

func TestRedisCacheClient_Set(t *testing.T) {
	db, mock := redismock.NewClientMock()
	client := NewRedisCacheClient(db, "10m")

	t.Run("success", func(t *testing.T) {
		key := "test_key"
		val := testValConstant
		ttl := time.Minute

		bytes, _ := json.Marshal(val)
		mock.ExpectSet(key, bytes, ttl).SetVal("OK")

		err := client.Set(context.Background(), key, val, ttl)
		require.NoError(t, err)
	})

	t.Run("redis error", func(t *testing.T) {
		key := "test_key_err"
		val := testValConstant
		ttl := time.Minute

		bytes, _ := json.Marshal(val)
		mock.ExpectSet(key, bytes, ttl).SetErr(errors.New("db disconnect"))

		err := client.Set(context.Background(), key, val, ttl)
		require.Error(t, err)
	})

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisCacheClient_Get(t *testing.T) {
	db, mock := redismock.NewClientMock()
	client := NewRedisCacheClient(db, "10m")

	key := "test_key"
	ctx := context.Background()

	t.Run("Set and Get success", func(t *testing.T) {
		val := testValConstant
		bytes, _ := json.Marshal(val)
		mock.ExpectSet(key, bytes, 10*time.Minute).SetVal("OK")
		mock.ExpectGet(key).SetVal(string(bytes))

		err := client.Set(ctx, key, val, 10*time.Minute)
		require.NoError(t, err)

		var res string
		err = client.Get(ctx, key, &res)
		require.NoError(t, err)
		assert.Equal(t, val, res)
	})

	t.Run("success struct", func(t *testing.T) {
		type user struct {
			ID   int
			Name string
		}
		key := "user_key"
		u := user{ID: 1, Name: "Alice"}
		bytes, _ := json.Marshal(u)

		mock.ExpectGet(key).SetVal(string(bytes))

		var dest user
		err := client.Get(context.Background(), key, &dest)
		require.NoError(t, err)
		assert.Equal(t, u, dest)
	})

	t.Run("cache miss", func(t *testing.T) {
		key := "missing_key"
		mock.ExpectGet(key).SetErr(redis.Nil)

		var dest string
		err := client.Get(context.Background(), key, &dest)
		require.ErrorIs(t, err, types.ErrCacheMiss)
	})

	t.Run("unmarshal error", func(t *testing.T) {
		key := "bad_json"
		mock.ExpectGet(key).SetVal("{invalid-json}")

		var dest map[string]any
		err := client.Get(context.Background(), key, &dest)
		require.Error(t, err)
	})

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisCacheClient_Del(t *testing.T) {
	db, mock := redismock.NewClientMock()
	client := NewRedisCacheClient(db, "10m")

	t.Run("success", func(t *testing.T) {
		key := "key_to_del"
		mock.ExpectDel(key).SetVal(1)

		err := client.Del(context.Background(), key)
		require.NoError(t, err)
	})

	t.Run("redis error", func(t *testing.T) {
		key := "key_err"
		mock.ExpectDel(key).SetErr(errors.New("some error"))

		err := client.Del(context.Background(), key)
		require.Error(t, err)
	})

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisCacheClient_IncrAndDecr(t *testing.T) {
	db, mock := redismock.NewClientMock()
	client := NewRedisCacheClient(db, "10m")

	t.Run("incr success", func(t *testing.T) {
		key := "counter"
		mock.ExpectIncrBy(key, 5).SetVal(105)

		val, err := client.Incr(context.Background(), key, 5)
		require.NoError(t, err)
		assert.Equal(t, int64(105), val)
	})

	t.Run("decr success", func(t *testing.T) {
		key := "counter"
		mock.ExpectDecrBy(key, 3).SetVal(102)

		val, err := client.Decr(context.Background(), key, 3)
		require.NoError(t, err)
		assert.Equal(t, int64(102), val)
	})

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisCacheClient_SetMany(t *testing.T) {
	db, mock := redismock.NewClientMock()
	client := NewRedisCacheClient(db, "10m")

	t.Run("success", func(t *testing.T) {
		items := []types.ClientBatchItem{
			{Key: "k1", Value: "v1", Expiration: time.Minute},
			{Key: "k2", Value: "v2", Expiration: time.Minute},
		}

		// SetMany logic iterates items and calls pipeline.Set()
		// redismock expects these calls in order.
		bytes1, _ := json.Marshal("v1")
		mock.ExpectSet("k1", bytes1, time.Minute).SetVal("OK")

		bytes2, _ := json.Marshal("v2")
		mock.ExpectSet("k2", bytes2, time.Minute).SetVal("OK")

		err := client.SetMany(context.Background(), items)
		require.NoError(t, err)
	})

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisCacheClient_TTL(t *testing.T) {
	db, mock := redismock.NewClientMock()
	client := NewRedisCacheClient(db, "10m")

	t.Run("found", func(t *testing.T) {
		mock.ExpectTTL("k").SetVal(time.Minute)

		ttl, err := client.TTL(context.Background(), "k")
		require.NoError(t, err)
		assert.Equal(t, time.Minute, ttl)
	})

	t.Run("missing", func(t *testing.T) {
		mock.ExpectTTL("missing").SetVal(-2 * time.Second)

		_, err := client.TTL(context.Background(), "missing")
		require.ErrorIs(t, err, types.ErrCacheMiss)
	})

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisCacheClient_ExpireNX(t *testing.T) {
	db, mock := redismock.NewClientMock()
	client := NewRedisCacheClient(db, "10m")

	t.Run("success", func(t *testing.T) {
		mock.ExpectExpireNX("k", time.Minute).SetVal(true)

		ok, err := client.ExpireNX(context.Background(), "k", time.Minute)
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("redis error", func(t *testing.T) {
		mock.ExpectExpireNX("k", time.Minute).SetErr(errors.New("db disconnect"))

		ok, err := client.ExpireNX(context.Background(), "k", time.Minute)
		require.Error(t, err)
		assert.False(t, ok)
	})

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisCacheClient_Set_JSONError(t *testing.T) {
	db, _ := redismock.NewClientMock()
	client := NewRedisCacheClient(db, "10m")

	// Channel cannot be marshaled to JSON
	err := client.Set(context.Background(), "k", make(chan int), time.Minute)
	require.Error(t, err)
}

func TestRedisCacheClient_SetMany_Empty(t *testing.T) {
	db, _ := redismock.NewClientMock()
	client := NewRedisCacheClient(db, "10m")
	err := client.SetMany(context.Background(), nil)
	require.NoError(t, err)
}

func TestRedisCacheClient_Get_InvalidDestination(t *testing.T) {
	db, mock := redismock.NewClientMock()
	client := NewRedisCacheClient(db, "10m")

	val := "some-value"
	bytes, _ := json.Marshal(val)

	// Implementation does check redis first
	mock.ExpectGet("k").SetVal(string(bytes))
	err := client.Get(context.Background(), "k", nil)
	require.ErrorIs(t, err, types.ErrInvalidDestination)

	mock.ExpectGet("k").SetVal(string(bytes))
	var strVal string
	err = client.Get(context.Background(), "k", strVal) // pass by value, not pointer
	require.ErrorIs(t, err, types.ErrInvalidDestination)
}

func TestRedisCacheClient_Get_ComplexPointers(t *testing.T) {
	db, mock := redismock.NewClientMock()
	client := NewRedisCacheClient(db, "10m")

	t.Run("double pointer", func(t *testing.T) {
		val := "test"
		bytes, _ := json.Marshal(val)
		mock.ExpectGet("k").SetVal(string(bytes))

		var ptr *string
		err := client.Get(context.Background(), "k", &ptr)
		require.NoError(t, err)
		require.NotNil(t, ptr)
		assert.Equal(t, "test", *ptr)
	})

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRedisCacheClient_ExpireNX_FalseCases(t *testing.T) {
	db, mock := redismock.NewClientMock()
	client := NewRedisCacheClient(db, "10m")

	t.Run("key missing", func(t *testing.T) {
		mock.ExpectExpireNX("k", time.Minute).SetVal(false)
		mock.ExpectTTL("k").SetVal(-2 * time.Second) // -2s = missing

		ok, err := client.ExpireNX(context.Background(), "k", time.Minute)
		require.ErrorIs(t, err, types.ErrCacheMiss)
		assert.False(t, ok)
	})

	t.Run("key exists with TTL", func(t *testing.T) {
		mock.ExpectExpireNX("k", time.Minute).SetVal(false)
		mock.ExpectTTL("k").SetVal(time.Second)

		ok, err := client.ExpireNX(context.Background(), "k", time.Minute)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("ttl error", func(t *testing.T) {
		mock.ExpectExpireNX("k", time.Minute).SetVal(false)
		mock.ExpectTTL("k").SetErr(errors.New("ttl fail"))

		ok, err := client.ExpireNX(context.Background(), "k", time.Minute)
		require.Error(t, err)
		assert.False(t, ok)
	})

	require.NoError(t, mock.ExpectationsWereMet())
}
