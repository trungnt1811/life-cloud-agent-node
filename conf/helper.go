package conf

import "strings"

func GetConfiguration() *Configuration {
	return &configuration
}

func GetRedisConfiguration() *RedisConfiguration {
	return &configuration.Redis
}

func GetCacheType() string {
	return configuration.CacheType
}

func GetAppName() string {
	return configuration.AppName
}

func IsDebugMode() bool {
	return configuration.Env == "development"
}

func GetDatabaseConfiguration() *DatabaseConfiguration {
	return &configuration.Database
}

func GetLogLevel() string {
	return configuration.LogLevel
}

// IsAutoMigrateEnabled returns whether auto-migration is enabled
func IsAutoMigrateEnabled() bool {
	return configuration.EnableAutoMigrate
}

// GetCORSAllowedOrigins returns the CORS allowed origins as a slice of strings
func GetCORSAllowedOrigins() []string {
	if configuration.CORS.AllowedOrigins == "" {
		return []string{"*"}
	}
	origins := strings.Split(configuration.CORS.AllowedOrigins, ",")
	result := make([]string, 0, len(origins))
	for _, origin := range origins {
		trimmed := strings.TrimSpace(origin)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	// If no valid origins found, allow all
	if len(result) == 0 {
		return []string{"*"}
	}
	return result
}

// GetCORSAllowedHeaders returns the CORS allowed headers as a comma-separated string
func GetCORSAllowedHeaders() string {
	if configuration.CORS.AllowedHeaders == "" {
		return "Content-Type,Content-Length,Accept-Encoding,X-CSRF-Token,Authorization,accept,origin,Cache-Control,X-Requested-With"
	}
	return configuration.CORS.AllowedHeaders
}
