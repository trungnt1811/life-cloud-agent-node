package runtimeconfig

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/go-backend-template/conf"
)

func TestModuleConfigsFromConfigurationUsesConfDefaults(t *testing.T) {
	defaults := conf.DefaultConfiguration()

	got := ModuleConfigsFromConfiguration(nil)

	require.Equal(t, defaults.AppName, got.Cache.AppName)
	require.Equal(t, defaults.CacheType, got.Cache.Type)
	require.Equal(t, defaults.Redis.RedisAddress, got.Cache.Redis.Address)
	require.Equal(t, defaults.Redis.RedisPassword, got.Cache.Redis.Password)
	require.Equal(t, defaults.Redis.RedisTtl, got.Cache.Redis.TTL)
	require.Equal(t, defaults.Redis.PoolSize, got.Cache.Redis.PoolSize)
	require.Equal(t, defaults.Database.DBUser, got.Database.User)
	require.Equal(t, defaults.Database.DBPassword, got.Database.Password)
	require.Equal(t, defaults.Database.DBHost, got.Database.Host)
	require.Equal(t, defaults.Database.DBPort, got.Database.Port)
	require.Equal(t, defaults.Database.DBName, got.Database.Name)
	require.Equal(t, defaults.Database.DBMaxOpenConns, got.Database.MaxOpenConns)
	require.Equal(t, defaults.Database.DBMaxIdleConns, got.Database.MaxIdleConns)
	require.Equal(t, defaults.Database.DBConnMaxLifetimeInMinute, got.Database.ConnMaxLifetimeInMinute)
}

func TestModuleConfigsFromConfigurationNormalizesPartialConfig(t *testing.T) {
	got := ModuleConfigsFromConfiguration(&conf.Configuration{
		AppName:   " api ",
		CacheType: " REDIS ",
		Redis: conf.RedisConfiguration{
			RedisAddress:  " cache ",
			RedisPassword: " redis-secret ",
		},
		Database: conf.DatabaseConfiguration{
			DBUser:                    " app ",
			DBPassword:                "secret",
			DBHost:                    " db ",
			DBPort:                    " 5433 ",
			DBName:                    " app_db ",
			DBMaxOpenConns:            40,
			DBMaxIdleConns:            12,
			DBConnMaxLifetimeInMinute: 7,
		},
	})

	require.Equal(t, "api", got.Cache.AppName)
	require.Equal(t, "redis", got.Cache.Type)
	require.Equal(t, "cache", got.Cache.Redis.Address)
	require.Equal(t, "redis-secret", got.Cache.Redis.Password)
	require.Equal(t, "app", got.Database.User)
	require.Equal(t, "secret", got.Database.Password)
	require.Equal(t, "db", got.Database.Host)
	require.Equal(t, "5433", got.Database.Port)
	require.Equal(t, "app_db", got.Database.Name)
	require.Equal(t, 40, got.Database.MaxOpenConns)
	require.Equal(t, 12, got.Database.MaxIdleConns)
	require.Equal(t, 7, got.Database.ConnMaxLifetimeInMinute)
}
