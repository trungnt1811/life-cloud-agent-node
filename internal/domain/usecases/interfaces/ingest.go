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
	// AnomalyReasons carries one "file:row:reason" entry per anomaly; its
	// count *is* the anomaly count, kept as one field so the two can never
	// drift apart the way a separately-tracked counter could.
	AnomalyReasons []string
}

// AnomalyCount returns the number of anomalies recorded for this run.
func (r *IngestResult) AnomalyCount() int {
	if r == nil {
		return 0
	}
	return len(r.AnomalyReasons)
}

// IngestHospitalExportUseCase persists normalized hospital exports into D3.
type IngestHospitalExportUseCase interface {
	IngestFiles(ctx context.Context, profile string, paths []string) (*IngestResult, error)
}
