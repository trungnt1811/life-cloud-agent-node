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
	require.Equal(t, defaultJobChunkSize, config.JobChunkSize)
	require.Equal(t, defaultJobChunkSize, NormalizeConfiguration(Configuration{JobChunkSize: -3}).JobChunkSize)
	require.Equal(t, 50, NormalizeConfiguration(Configuration{JobChunkSize: 50}).JobChunkSize)
}

// The gRPC client fields (decision 0006) are blank/false by default, same as
// ADMIN_BASIC_AUTH_USER/PASS - "not configured" is a valid deployment
// (control center not dialed yet), not a value to default away from.
func TestNormalizeConfigurationLeavesControlCenterFieldsUnset(t *testing.T) {
	config := NormalizeConfiguration(Configuration{})

	require.Empty(t, config.ControlCenterAddress)
	require.Empty(t, config.NodeID)
	require.Empty(t, config.AgentVersion)
	require.False(t, config.ControlCenterInsecure)
	require.Empty(t, config.ControlCenterCAFile)
	require.Empty(t, config.ControlCenterClientCertFile)
	require.Empty(t, config.ControlCenterClientKeyFile)
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
	require.Equal(t, defaultJobChunkSize, defaultConfigurations["JOB_CHUNK_SIZE"])
}

func TestProductionFederatedClientRequiresMTLS(t *testing.T) {
	config := DefaultConfiguration()
	config.Env = "production"
	config.ControlCenterAddress = "control-plane:9090"
	config.ControlCenterInsecure = true
	require.ErrorContains(t, ValidateConfiguration(config), "CONTROL_CENTER_INSECURE")
	config.ControlCenterAddress = ""
	require.ErrorContains(t, ValidateConfiguration(config), "CONTROL_CENTER_INSECURE")
	config.ControlCenterAddress = "control-plane:9090"

	config.ControlCenterInsecure = false
	require.ErrorContains(t, ValidateConfiguration(config), "CONTROL_CENTER_CLIENT_CERT_FILE")

	config.ControlCenterClientCertFile = "node.pem"
	config.ControlCenterClientKeyFile = "node-key.pem"
	require.NoError(t, ValidateConfiguration(config))
}
