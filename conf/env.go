package conf

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/spf13/viper"

	"github.com/lifenetwork-ai/life-cloud-agent-node/constants"
)

type DatabaseConfiguration struct {
	DBUser                    string `mapstructure:"DB_USER"`
	DBPassword                string `mapstructure:"DB_PASSWORD"`
	DBHost                    string `mapstructure:"DB_HOST"`
	DBPort                    string `mapstructure:"DB_PORT"`
	DBName                    string `mapstructure:"DB_NAME"`
	DBMaxOpenConns            int    `mapstructure:"DB_MAX_OPEN_CONNS"`
	DBMaxIdleConns            int    `mapstructure:"DB_MAX_IDLE_CONNS"`
	DBConnMaxLifetimeInMinute int    `mapstructure:"DB_CONN_MAX_LIFETIME_IN_MINUTE"`
}

type RedisConfiguration struct {
	RedisAddress   string `mapstructure:"REDIS_ADDRESS"`
	RedisPassword  string `mapstructure:"REDIS_PASSWORD"`
	RedisTtl       string `mapstructure:"REDIS_TTL"`
	PoolSize       int    `mapstructure:"REDIS_POOL_SIZE"`
	MinIdleConns   int    `mapstructure:"REDIS_MIN_IDLE_CONNS"`
	DialTimeoutMs  int    `mapstructure:"REDIS_DIAL_TIMEOUT_MS"`
	ReadTimeoutMs  int    `mapstructure:"REDIS_READ_TIMEOUT_MS"`
	WriteTimeoutMs int    `mapstructure:"REDIS_WRITE_TIMEOUT_MS"`
	PoolTimeoutMs  int    `mapstructure:"REDIS_POOL_TIMEOUT_MS"`
}

type CORSConfiguration struct {
	AllowedOrigins string `mapstructure:"CORS_ALLOWED_ORIGINS"`
	AllowedHeaders string `mapstructure:"CORS_ALLOWED_HEADERS"`
}

type Configuration struct {
	Database             DatabaseConfiguration `mapstructure:",squash"`
	Redis                RedisConfiguration    `mapstructure:",squash"`
	CORS                 CORSConfiguration     `mapstructure:",squash"`
	AppName              string                `mapstructure:"APP_NAME"`
	AppPort              uint32                `mapstructure:"APP_PORT"`
	AppRequestTimeoutMs  int                   `mapstructure:"APP_REQUEST_TIMEOUT_MS"`
	Env                  string                `mapstructure:"ENV"`
	LogLevel             string                `mapstructure:"LOG_LEVEL"`
	CacheType            string                `mapstructure:"CACHE_TYPE"`
	EnableAutoMigrate    bool                  `mapstructure:"ENABLE_AUTO_MIGRATE"`
	SwaggerBasicAuthUser string                `mapstructure:"SWAGGER_BASIC_AUTH_USER"`
	SwaggerBasicAuthPass string                `mapstructure:"SWAGGER_BASIC_AUTH_PASS"`
	// AdminBasicAuthUser/Pass gate node-local admin routes (e.g.
	// /admin/query-fields). Deliberately separate from the Swagger
	// credentials: rotating docs access must not also rotate the ability
	// to mutate which clinical fields this node exposes, and vice versa.
	AdminBasicAuthUser string `mapstructure:"ADMIN_BASIC_AUTH_USER"`
	AdminBasicAuthPass string `mapstructure:"ADMIN_BASIC_AUTH_PASS"`
	// SuppressionThreshold is the node-local small-cell threshold (process
	// 3.6). When 0 < matching_count < threshold the visible count is zero
	// and suppressed=true. Default is 5. Suppression fails closed: 0 (unset
	// or explicitly "0") is replaced by the default, never treated as
	// "disabled". A threshold of 1 hides nothing, for tests.
	SuppressionThreshold uint64 `mapstructure:"SUPPRESSION_THRESHOLD"`
	// JobChunkSize is how many candidate patients one resumable, checkpointed
	// cohort chunk covers (decision 0005). Below 1 (unset) means the default.
	JobChunkSize int `mapstructure:"JOB_CHUNK_SIZE"`
	// ControlCenterAddress is the control center's gRPC host:port. Blank
	// disables the federated client worker entirely (no control center to
	// dial yet is a valid deployment - decision 0001).
	ControlCenterAddress string `mapstructure:"CONTROL_CENTER_ADDRESS"`
	// NodeID identifies this node to the control center's registry (D2).
	NodeID string `mapstructure:"NODE_ID"`
	// AgentVersion is reported on every (re)connect so the control center's
	// registry reflects what is actually deployed at this site (decision 0003).
	AgentVersion string `mapstructure:"AGENT_VERSION"`
	// ControlCenterInsecure opts out of TLS. Decision 0006: TLS is required by
	// default; this exists only for local development and the Phase 8 stub.
	ControlCenterInsecure bool `mapstructure:"CONTROL_CENTER_INSECURE"`
	// ControlCenterCAFile verifies the control center's certificate against a
	// private CA instead of the system pool. Blank uses the system pool.
	ControlCenterCAFile string `mapstructure:"CONTROL_CENTER_CA_FILE"`
	// ControlCenterClientCertFile/KeyFile present this node's client
	// certificate (mTLS). Decision 0006: setting exactly one is a startup
	// error, matching the ADMIN_BASIC_AUTH_USER/PASS pattern.
	ControlCenterClientCertFile string `mapstructure:"CONTROL_CENTER_CLIENT_CERT_FILE"`
	ControlCenterClientKeyFile  string `mapstructure:"CONTROL_CENTER_CLIENT_KEY_FILE"`
	GovernanceSyncEnabled       bool   `mapstructure:"GOVERNANCE_SYNC_ENABLED"`
	GovernedExecutionEnabled    bool   `mapstructure:"GOVERNED_EXECUTION_ENABLED"`
}

var configuration = NormalizeConfiguration(Configuration{})

const (
	defaultAppName                     = "life-cloud-agent-node"
	defaultAppPort              uint32 = 8080
	defaultEnv                         = "development"
	defaultLogLevel                    = "info"
	defaultRedisAddress                = "localhost:6379"
	defaultRedisPassword               = ""
	defaultRedisTTL                    = "10m"
	defaultRedisPoolSize               = 50
	defaultRedisMinIdleConns           = 5
	defaultRedisTimeoutMs              = 1000
	defaultDBUser                      = "postgres"
	defaultDBPassword                  = "postgres"
	defaultDBHost                      = "localhost"
	defaultDBPort                      = "5432"
	defaultDBName                      = "life_cloud_agent_node"
	defaultDBMaxIdleConns              = 5
	defaultDBMaxOpenConns              = 25
	defaultDBConnMaxLifetimeMin        = 60
	defaultCORSAllowedOrigins          = "*"
	defaultCORSAllowedHeaders          = "Content-Type,Content-Length,Accept-Encoding,X-CSRF-Token,Authorization,accept,origin,Cache-Control,X-Requested-With,X-Request-ID,X-Correlation-ID"
	defaultSuppressionThreshold uint64 = constants.DefaultSuppressionThreshold
	defaultJobChunkSize                = constants.DefaultJobChunkSize
)

// NOTE: when adding a new env, add it here and expose only scoped config to modules.
var defaultConfigurations = map[string]any{
	"APP_NAME":                        defaultAppName,
	"APP_PORT":                        defaultAppPort,
	"APP_REQUEST_TIMEOUT_MS":          constants.DefaultHTTPRequestMs,
	"CORS_ALLOWED_ORIGINS":            defaultCORSAllowedOrigins,
	"CORS_ALLOWED_HEADERS":            defaultCORSAllowedHeaders,
	"ENV_FILE":                        ".env",
	"ENV":                             defaultEnv,
	"LOG_LEVEL":                       defaultLogLevel,
	"CACHE_TYPE":                      constants.CacheTypeInMemory,
	"REDIS_ADDRESS":                   defaultRedisAddress,
	"REDIS_PASSWORD":                  defaultRedisPassword,
	"REDIS_TTL":                       defaultRedisTTL,
	"REDIS_POOL_SIZE":                 defaultRedisPoolSize,
	"REDIS_MIN_IDLE_CONNS":            defaultRedisMinIdleConns,
	"REDIS_DIAL_TIMEOUT_MS":           defaultRedisTimeoutMs,
	"REDIS_READ_TIMEOUT_MS":           defaultRedisTimeoutMs,
	"REDIS_WRITE_TIMEOUT_MS":          defaultRedisTimeoutMs,
	"REDIS_POOL_TIMEOUT_MS":           defaultRedisTimeoutMs,
	"DB_USER":                         defaultDBUser,
	"DB_PASSWORD":                     defaultDBPassword,
	"DB_HOST":                         defaultDBHost,
	"DB_PORT":                         defaultDBPort,
	"DB_NAME":                         defaultDBName,
	"DB_MAX_IDLE_CONNS":               defaultDBMaxIdleConns,
	"DB_MAX_OPEN_CONNS":               defaultDBMaxOpenConns,
	"DB_CONN_MAX_LIFETIME_IN_MINUTE":  defaultDBConnMaxLifetimeMin,
	"ENABLE_AUTO_MIGRATE":             "false",
	"SWAGGER_BASIC_AUTH_USER":         "",
	"SWAGGER_BASIC_AUTH_PASS":         "",
	"ADMIN_BASIC_AUTH_USER":           "",
	"ADMIN_BASIC_AUTH_PASS":           "",
	"SUPPRESSION_THRESHOLD":           defaultSuppressionThreshold,
	"JOB_CHUNK_SIZE":                  defaultJobChunkSize,
	"CONTROL_CENTER_ADDRESS":          "",
	"NODE_ID":                         "",
	"AGENT_VERSION":                   "",
	"CONTROL_CENTER_INSECURE":         "false",
	"CONTROL_CENTER_CA_FILE":          "",
	"CONTROL_CENTER_CLIENT_CERT_FILE": "",
	"CONTROL_CENTER_CLIENT_KEY_FILE":  "",
	"GOVERNANCE_SYNC_ENABLED":         false,
	"GOVERNED_EXECUTION_ENABLED":      false,
}

// DefaultConfiguration returns the normalized config used when no env is loaded.
func DefaultConfiguration() Configuration {
	return NormalizeConfiguration(Configuration{})
}

// LoadConfig returns the loaded configuration.
func LoadConfig() (*Configuration, error) {
	envFile := os.Getenv("ENV_FILE")
	if envFile == "" {
		envFile = ".env"
	}
	return LoadConfigFromEnvFile(envFile)
}

// LoadConfigFromEnvFile loads config from a dotenv file, environment variables, and defaults.
func LoadConfigFromEnvFile(envFile string) (*Configuration, error) {
	reader := viper.New()
	reader.SetConfigFile(envFile)
	reader.SetConfigType("env")
	reader.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	reader.AutomaticEnv()
	loadDefaultConfigs(reader)

	if err := reader.ReadInConfig(); err == nil {
		log.Printf("Loaded configuration from file: %s", envFile)
	} else {
		log.Printf("Config file %q not found or unreadable, using environment variables and defaults", envFile)
	}

	var loaded Configuration
	if err := reader.Unmarshal(&loaded); err != nil {
		return nil, err
	}
	loaded = NormalizeConfiguration(loaded)
	if err := ValidateConfiguration(loaded); err != nil {
		return nil, err
	}
	configuration = loaded
	log.Println("Configuration loaded successfully")
	return &configuration, nil
}

func ValidateConfiguration(config Configuration) error {
	config = NormalizeConfiguration(config)
	if config.GovernedExecutionEnabled && !config.GovernanceSyncEnabled {
		return fmt.Errorf("GOVERNED_EXECUTION_ENABLED requires GOVERNANCE_SYNC_ENABLED and verified mutual TLS")
	}
	if config.GovernanceSyncEnabled && (strings.TrimSpace(config.ControlCenterAddress) == "" || config.ControlCenterInsecure || strings.TrimSpace(config.ControlCenterClientCertFile) == "" || strings.TrimSpace(config.ControlCenterClientKeyFile) == "") {
		return fmt.Errorf("GOVERNANCE_SYNC_ENABLED requires CONTROL_CENTER_ADDRESS and verified TLS with CONTROL_CENTER_CLIENT_CERT_FILE/KEY_FILE")
	}
	if !isProductionEnv(config.Env) {
		return nil
	}
	if config.ControlCenterInsecure {
		return fmt.Errorf("CONTROL_CENTER_INSECURE must be false in production")
	}
	if strings.TrimSpace(config.ControlCenterAddress) == "" {
		return nil
	}
	if strings.TrimSpace(config.ControlCenterClientCertFile) == "" || strings.TrimSpace(config.ControlCenterClientKeyFile) == "" {
		return fmt.Errorf("CONTROL_CENTER_CLIENT_CERT_FILE and CONTROL_CENTER_CLIENT_KEY_FILE are required in production")
	}
	return nil
}

func isProductionEnv(env string) bool {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "prod", "production":
		return true
	default:
		return false
	}
}

// NormalizeConfiguration applies cross-field defaults after env unmarshalling.
func NormalizeConfiguration(config Configuration) Configuration {
	config.AppName = strings.TrimSpace(config.AppName)
	if config.AppName == "" {
		config.AppName = defaultAppName
	}
	if config.AppPort == 0 {
		config.AppPort = defaultAppPort
	}
	config.Env = strings.TrimSpace(config.Env)
	if config.Env == "" {
		config.Env = defaultEnv
	}
	config.LogLevel = strings.TrimSpace(config.LogLevel)
	if config.LogLevel == "" {
		config.LogLevel = defaultLogLevel
	}
	config.CacheType = strings.TrimSpace(config.CacheType)
	if config.CacheType == "" {
		config.CacheType = constants.CacheTypeInMemory
	}
	if config.AppRequestTimeoutMs <= 0 {
		config.AppRequestTimeoutMs = constants.DefaultHTTPRequestMs
	}
	config.CORS.AllowedOrigins = strings.TrimSpace(config.CORS.AllowedOrigins)
	if config.CORS.AllowedOrigins == "" {
		config.CORS.AllowedOrigins = defaultCORSAllowedOrigins
	}
	config.CORS.AllowedHeaders = strings.TrimSpace(config.CORS.AllowedHeaders)
	if config.CORS.AllowedHeaders == "" {
		config.CORS.AllowedHeaders = defaultCORSAllowedHeaders
	}
	config.Redis.RedisAddress = strings.TrimSpace(config.Redis.RedisAddress)
	if config.Redis.RedisAddress == "" {
		config.Redis.RedisAddress = defaultRedisAddress
	}
	config.Redis.RedisPassword = strings.TrimSpace(config.Redis.RedisPassword)
	config.Redis.RedisTtl = strings.TrimSpace(config.Redis.RedisTtl)
	if config.Redis.RedisTtl == "" {
		config.Redis.RedisTtl = defaultRedisTTL
	}
	if config.Redis.PoolSize <= 0 {
		config.Redis.PoolSize = defaultRedisPoolSize
	}
	if config.Redis.MinIdleConns <= 0 {
		config.Redis.MinIdleConns = defaultRedisMinIdleConns
	}
	if config.Redis.DialTimeoutMs <= 0 {
		config.Redis.DialTimeoutMs = defaultRedisTimeoutMs
	}
	if config.Redis.ReadTimeoutMs <= 0 {
		config.Redis.ReadTimeoutMs = defaultRedisTimeoutMs
	}
	if config.Redis.WriteTimeoutMs <= 0 {
		config.Redis.WriteTimeoutMs = defaultRedisTimeoutMs
	}
	if config.Redis.PoolTimeoutMs <= 0 {
		config.Redis.PoolTimeoutMs = defaultRedisTimeoutMs
	}
	config.Database.DBUser = strings.TrimSpace(config.Database.DBUser)
	if config.Database.DBUser == "" {
		config.Database.DBUser = defaultDBUser
	}
	if config.Database.DBPassword == "" {
		config.Database.DBPassword = defaultDBPassword
	}
	config.Database.DBHost = strings.TrimSpace(config.Database.DBHost)
	if config.Database.DBHost == "" {
		config.Database.DBHost = defaultDBHost
	}
	config.Database.DBPort = strings.TrimSpace(config.Database.DBPort)
	if config.Database.DBPort == "" {
		config.Database.DBPort = defaultDBPort
	}
	config.Database.DBName = strings.TrimSpace(config.Database.DBName)
	if config.Database.DBName == "" {
		config.Database.DBName = defaultDBName
	}
	if config.Database.DBMaxIdleConns <= 0 {
		config.Database.DBMaxIdleConns = defaultDBMaxIdleConns
	}
	if config.Database.DBMaxOpenConns <= 0 {
		config.Database.DBMaxOpenConns = defaultDBMaxOpenConns
	}
	if config.Database.DBConnMaxLifetimeInMinute <= 0 {
		config.Database.DBConnMaxLifetimeInMinute = defaultDBConnMaxLifetimeMin
	}
	if config.SuppressionThreshold == 0 {
		config.SuppressionThreshold = defaultSuppressionThreshold
	}
	if config.JobChunkSize < 1 {
		config.JobChunkSize = defaultJobChunkSize
	}
	return config
}

// loadDefaultConfigs sets default values for critical configurations.
func loadDefaultConfigs(reader *viper.Viper) {
	if reader == nil {
		return
	}
	for configKey, configValue := range defaultConfigurations {
		reader.SetDefault(configKey, configValue)
	}
}
