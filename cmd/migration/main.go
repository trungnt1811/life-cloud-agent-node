package main

import (
	"log"
	"os"

	"github.com/lifenetwork-ai/life-cloud-agent-node/conf"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/postgres"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/di/instances"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/runtimeconfig"
)

func main() {
	config, err := conf.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	moduleConfigs := runtimeconfig.ModuleConfigsFromConfiguration(config)
	db := instances.DBInstance(moduleConfigs.Database)

	// Get migration scripts path (default or CLI argument)
	basePath := "internal/adapters/postgres/scripts"
	if len(os.Args) > 1 {
		basePath = os.Args[1] // Allow passing a migration path
	}

	log.Printf("Running database migrations from: %s", basePath)

	// Run migrations
	if err := postgres.RunMigrations(db, basePath); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	log.Println("Database migration completed successfully.")
}
