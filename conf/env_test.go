package conf

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/life-cloud-agent-node/constants"
)

func TestNormalizeConfigurationUsesRuntimeDefaults(t *testing.T) {
	config := NormalizeConfiguration(Configuration{})

	require.Equal(t, defaultAppName, config.AppName)
	require.Equal(t, defaultAppPort, config.AppPort)
	require.Equal(t, defaultEnv, config.Env)
	require.Equal(t, defaultLogLevel, config.LogLevel)
	require.Equal(t, constants.CacheTypeInMemory, config.CacheType)
	require.Equal(t, constants.DefaultHTTPRequestMs, config.AppRequestTimeoutMs)
	require.Equal(t, defaultRedisAddress, config.Redis.RedisAddress)
	require.Equal(t, defaultRedisPassword, config.Redis.RedisPassword)
	require.Equal(t, defaultRedisTTL, config.Redis.RedisTtl)
	require.Equal(t, defaultRedisPoolSize, config.Redis.PoolSize)
	require.Equal(t, defaultDBUser, config.Database.DBUser)
	require.Equal(t, defaultDBPassword, config.Database.DBPassword)
	require.Equal(t, defaultDBHost, config.Database.DBHost)
	require.Equal(t, defaultDBPort, config.Database.DBPort)
	require.Equal(t, defaultDBName, config.Database.DBName)
	require.Equal(t, defaultDBMaxOpenConns, config.Database.DBMaxOpenConns)
	require.Equal(t, defaultDBMaxIdleConns, config.Database.DBMaxIdleConns)
	require.Equal(t, defaultDBConnMaxLifetimeMin, config.Database.DBConnMaxLifetimeInMinute)
	require.Equal(t, defaultSuppressionThreshold, config.SuppressionThreshold)
}

func TestDefaultConfigurationMapUsesRuntimeDefaults(t *testing.T) {
	require.Equal(t, defaultAppName, defaultConfigurations["APP_NAME"])
	require.Equal(t, defaultAppPort, defaultConfigurations["APP_PORT"])
	require.Equal(t, defaultEnv, defaultConfigurations["ENV"])
	require.Equal(t, defaultLogLevel, defaultConfigurations["LOG_LEVEL"])
	require.Equal(t, defaultDBName, defaultConfigurations["DB_NAME"])
	require.Equal(t, defaultDBPort, defaultConfigurations["DB_PORT"])
	require.Equal(t, defaultRedisPassword, defaultConfigurations["REDIS_PASSWORD"])
	require.Equal(t, defaultSuppressionThreshold, defaultConfigurations["SUPPRESSION_THRESHOLD"])
}
