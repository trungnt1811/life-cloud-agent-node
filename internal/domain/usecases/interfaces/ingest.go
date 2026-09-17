package interfaces

import (
	"context"

	domaintypes "github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
)

// HospitalExportAdapter parses one hospital-profile export file into D3-ready rows.
type HospitalExportAdapter interface {
	ParseFile(path string) ([]domaintypes.NormalizedSpecimen, []domaintypes.Anomaly, error)
}

// IngestResult summarizes one ingestion run.
type IngestResult struct {
	Profile           string
	Files             int
	SpecimensParsed   int
	SpecimensSaved    int
	ObservationsSaved int
	Anomalies         int
	AnomalyReasons    []string
}

// IngestHospitalExportUseCase persists normalized hospital exports into D3.
type IngestHospitalExportUseCase interface {
	IngestFiles(ctx context.Context, profile string, paths []string) (*IngestResult, error)
}
