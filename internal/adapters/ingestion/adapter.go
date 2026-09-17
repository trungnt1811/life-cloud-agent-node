package ingestion

import (
	"fmt"
	"strings"

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

// AdapterForProfile resolves a hospital export adapter by profile flag.
func AdapterForProfile(profile string) (interfaces.HospitalExportAdapter, error) {
	adapters := DefaultAdapters()
	adapter := adapters[strings.ToUpper(strings.TrimSpace(profile))]
	if adapter == nil {
		return nil, fmt.Errorf("unsupported hospital export profile %q", profile)
	}
	return adapter, nil
}
