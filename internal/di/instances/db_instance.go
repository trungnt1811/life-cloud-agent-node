package instances

import (
	"database/sql"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/runtimeconfig"
)

var dbInstance *gorm.DB

// DBInstance returns a singleton instance of the database connection.
func DBInstance(config runtimeconfig.DatabaseConfig) *gorm.DB {
	if dbInstance == nil {
		config = runtimeconfig.NormalizeDatabaseConfig(config)
		dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
			config.Host, config.User, config.Password, config.Name, config.Port)
		db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err != nil {
			panic(fmt.Sprintf("Failed to connect to database: %v", err))
		}
		sqlDB, err := db.DB()
		if err != nil {
			panic(fmt.Sprintf("Failed to access database pool: %v", err))
		}
		applyDatabasePoolConfig(sqlDB, config)
		dbInstance = db
	}
	return dbInstance
}

func applyDatabasePoolConfig(db *sql.DB, config runtimeconfig.DatabaseConfig) {
	if db == nil {
		return
	}
	settings := databasePoolSettingsFromConfig(config)
	db.SetMaxOpenConns(settings.maxOpenConns)
	db.SetMaxIdleConns(settings.maxIdleConns)
	db.SetConnMaxLifetime(settings.connMaxLifetime)
}

type databasePoolSettings struct {
	maxOpenConns    int
	maxIdleConns    int
	connMaxLifetime time.Duration
}

func databasePoolSettingsFromConfig(config runtimeconfig.DatabaseConfig) databasePoolSettings {
	config = runtimeconfig.NormalizeDatabaseConfig(config)
	return databasePoolSettings{
		maxOpenConns:    config.MaxOpenConns,
		maxIdleConns:    config.MaxIdleConns,
		connMaxLifetime: time.Duration(config.ConnMaxLifetimeInMinute) * time.Minute,
	}
}
