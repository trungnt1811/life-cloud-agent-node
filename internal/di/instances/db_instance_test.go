package instances

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/go-backend-template/internal/runtimeconfig"
)

func TestDatabasePoolSettingsFromConfig(t *testing.T) {
	got := databasePoolSettingsFromConfig(runtimeconfig.DatabaseConfig{
		User:                    "postgres",
		Password:                "postgres",
		Host:                    "localhost",
		Port:                    "5432",
		Name:                    "app",
		MaxOpenConns:            40,
		MaxIdleConns:            12,
		ConnMaxLifetimeInMinute: 7,
	})

	require.Equal(t, 40, got.maxOpenConns)
	require.Equal(t, 12, got.maxIdleConns)
	require.Equal(t, 7*time.Minute, got.connMaxLifetime)
}
