package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/lifenetwork-ai/life-cloud-agent-node/conf"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/ingestion"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/di/instances"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/runtimeconfig"
)

func main() {
	profile := flag.String("profile", "", "hospital export profile: VN_A, VN_B, or VN_C")
	flag.Parse()
	paths := flag.Args()
	if strings.TrimSpace(*profile) == "" || len(paths) == 0 {
		fmt.Fprintf(os.Stderr, "usage: go run ./cmd/ingest -profile VN_A <export.csv> [more.csv...]\n")
		os.Exit(2)
	}

	config, err := conf.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	moduleConfigs := runtimeconfig.ModuleConfigsFromConfiguration(config)
	db := instances.DBInstance(moduleConfigs.Database)
	appLogger := logger.GetLogger()

	ucase := usecases.NewIngestHospitalExportUseCase(
		repositories.NewPatientRegistryRepository(db, appLogger),
		ingestion.DefaultAdapters(),
		appLogger,
	)

	result, err := ucase.IngestFiles(context.Background(), *profile, paths)
	if err != nil {
		log.Fatalf("Ingest failed: %v", err)
	}

	log.Printf(
		"ingest complete profile=%s files=%d specimens_saved=%d observations_saved=%d anomalies=%d",
		result.Profile,
		result.Files,
		result.SpecimensSaved,
		result.ObservationsSaved,
		result.Anomalies,
	)
}
