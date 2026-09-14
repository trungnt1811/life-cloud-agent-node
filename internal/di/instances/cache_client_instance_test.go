package instances

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/go-backend-template/internal/runtimeconfig"
)

func TestRedisOptionsFromConfigIncludesPassword(t *testing.T) {
	got := redisOptionsFromConfig(runtimeconfig.RedisConfig{
		Address:        "redis.internal:6379",
		Password:       "redis-secret",
		PoolSize:       40,
		MinIdleConns:   12,
		DialTimeoutMs:  300,
		ReadTimeoutMs:  400,
		WriteTimeoutMs: 500,
		PoolTimeoutMs:  600,
	})

	require.Equal(t, "redis.internal:6379", got.Addr)
	require.Equal(t, "redis-secret", got.Password)
	require.Equal(t, 40, got.PoolSize)
	require.Equal(t, 12, got.MinIdleConns)
	require.Equal(t, 300*time.Millisecond, got.DialTimeout)
	require.Equal(t, 400*time.Millisecond, got.ReadTimeout)
	require.Equal(t, 500*time.Millisecond, got.WriteTimeout)
	require.Equal(t, 600*time.Millisecond, got.PoolTimeout)
}
