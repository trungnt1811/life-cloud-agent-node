package ingestion

import (
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases/interfaces"
)

// Profile names accepted by the ingest use case / CLI.
const (
	ProfileVNA = "VN_A"
	ProfileVNB = "VN_B"
	ProfileVNC = "VN_C"
)

// DefaultAdapters returns the three life-cloud demo hospital profile adapters.
func DefaultAdapters() map[string]interfaces.HospitalExportAdapter {
	return map[string]interfaces.HospitalExportAdapter{
		ProfileVNA: NewVNAAdapter(),
		ProfileVNB: NewVNBAdapter(),
		ProfileVNC: NewVNCAdapter(),
	}
}
