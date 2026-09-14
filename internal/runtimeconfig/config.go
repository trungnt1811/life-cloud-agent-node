package runtimeconfig

import (
	"strings"

	"github.com/lifenetwork-ai/go-backend-template/conf"
)

type ModuleConfigs struct {
	Database DatabaseConfig
	Cache    CacheConfig
}

type DatabaseConfig struct {
	User                    string
	Password                string
	Host                    string
	Port                    string
	Name                    string
	MaxOpenConns            int
	MaxIdleConns            int
	ConnMaxLifetimeInMinute int
}

type CacheConfig struct {
	AppName string
	Type    string
	Redis   RedisConfig
}

type RedisConfig struct {
	Address        string
	Password       string
	TTL            string
	PoolSize       int
	MinIdleConns   int
	DialTimeoutMs  int
	ReadTimeoutMs  int
	WriteTimeoutMs int
	PoolTimeoutMs  int
}

func ModuleConfigsFromConfiguration(config *conf.Configuration) ModuleConfigs {
	if config == nil {
		defaults := conf.DefaultConfiguration()
		config = &defaults
	} else {
		normalized := conf.NormalizeConfiguration(*config)
		config = &normalized
	}
	return ModuleConfigs{
		Database: DatabaseConfig{
			User:                    strings.TrimSpace(config.Database.DBUser),
			Password:                config.Database.DBPassword,
			Host:                    strings.TrimSpace(config.Database.DBHost),
			Port:                    strings.TrimSpace(config.Database.DBPort),
			Name:                    strings.TrimSpace(config.Database.DBName),
			MaxOpenConns:            config.Database.DBMaxOpenConns,
			MaxIdleConns:            config.Database.DBMaxIdleConns,
			ConnMaxLifetimeInMinute: config.Database.DBConnMaxLifetimeInMinute,
		},
		Cache: CacheConfig{
			AppName: strings.TrimSpace(config.AppName),
			Type:    strings.ToLower(strings.TrimSpace(config.CacheType)),
			Redis: RedisConfig{
				Address:        strings.TrimSpace(config.Redis.RedisAddress),
				Password:       strings.TrimSpace(config.Redis.RedisPassword),
				TTL:            strings.TrimSpace(config.Redis.RedisTtl),
				PoolSize:       config.Redis.PoolSize,
				MinIdleConns:   config.Redis.MinIdleConns,
				DialTimeoutMs:  config.Redis.DialTimeoutMs,
				ReadTimeoutMs:  config.Redis.ReadTimeoutMs,
				WriteTimeoutMs: config.Redis.WriteTimeoutMs,
				PoolTimeoutMs:  config.Redis.PoolTimeoutMs,
			},
		},
	}
}

func NormalizeCacheConfig(config CacheConfig) CacheConfig {
	defaults := ModuleConfigsFromConfiguration(nil).Cache
	config.AppName = strings.TrimSpace(config.AppName)
	if config.AppName == "" {
		config.AppName = defaults.AppName
	}
	config.Type = strings.ToLower(strings.TrimSpace(config.Type))
	if config.Type == "" {
		config.Type = defaults.Type
	}
	config.Redis.Address = strings.TrimSpace(config.Redis.Address)
	if config.Redis.Address == "" {
		config.Redis.Address = defaults.Redis.Address
	}
	config.Redis.Password = strings.TrimSpace(config.Redis.Password)
	config.Redis.TTL = strings.TrimSpace(config.Redis.TTL)
	if config.Redis.TTL == "" {
		config.Redis.TTL = defaults.Redis.TTL
	}
	config.Redis = NormalizeRedisConfig(config.Redis)
	return config
}

func NormalizeRedisConfig(config RedisConfig) RedisConfig {
	defaults := ModuleConfigsFromConfiguration(nil).Cache.Redis
	config.Address = strings.TrimSpace(config.Address)
	if config.Address == "" {
		config.Address = defaults.Address
	}
	config.Password = strings.TrimSpace(config.Password)
	config.TTL = strings.TrimSpace(config.TTL)
	if config.TTL == "" {
		config.TTL = defaults.TTL
	}
	if config.PoolSize <= 0 {
		config.PoolSize = defaults.PoolSize
	}
	if config.MinIdleConns <= 0 {
		config.MinIdleConns = defaults.MinIdleConns
	}
	if config.DialTimeoutMs <= 0 {
		config.DialTimeoutMs = defaults.DialTimeoutMs
	}
	if config.ReadTimeoutMs <= 0 {
		config.ReadTimeoutMs = defaults.ReadTimeoutMs
	}
	if config.WriteTimeoutMs <= 0 {
		config.WriteTimeoutMs = defaults.WriteTimeoutMs
	}
	if config.PoolTimeoutMs <= 0 {
		config.PoolTimeoutMs = defaults.PoolTimeoutMs
	}
	return config
}

func NormalizeDatabaseConfig(config DatabaseConfig) DatabaseConfig {
	defaults := ModuleConfigsFromConfiguration(nil).Database
	config.User = strings.TrimSpace(config.User)
	if config.User == "" {
		config.User = defaults.User
	}
	if config.Password == "" {
		config.Password = defaults.Password
	}
	config.Host = strings.TrimSpace(config.Host)
	if config.Host == "" {
		config.Host = defaults.Host
	}
	config.Port = strings.TrimSpace(config.Port)
	if config.Port == "" {
		config.Port = defaults.Port
	}
	config.Name = strings.TrimSpace(config.Name)
	if config.Name == "" {
		config.Name = defaults.Name
	}
	if config.MaxOpenConns <= 0 {
		config.MaxOpenConns = defaults.MaxOpenConns
	}
	if config.MaxIdleConns <= 0 {
		config.MaxIdleConns = defaults.MaxIdleConns
	}
	if config.ConnMaxLifetimeInMinute <= 0 {
		config.ConnMaxLifetimeInMinute = defaults.ConnMaxLifetimeInMinute
	}
	return config
}
