package usecases

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/repositories"
	domaintypes "github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases/interfaces"
	loggerpkg "github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
)

type ingestHospitalExportUseCase struct {
	registry repositories.PatientRegistryRepository
	adapters map[string]interfaces.HospitalExportAdapter
	logger   loggerpkg.Logger
}

// NewIngestHospitalExportUseCase wires profile adapters into the ingest use case.
func NewIngestHospitalExportUseCase(
	registry repositories.PatientRegistryRepository,
	adapters map[string]interfaces.HospitalExportAdapter,
	logger loggerpkg.Logger,
) interfaces.IngestHospitalExportUseCase {
	if logger == nil {
		logger = loggerpkg.GetLogger()
	}
	copied := make(map[string]interfaces.HospitalExportAdapter, len(adapters))
	for profile, adapter := range adapters {
		copied[strings.ToUpper(strings.TrimSpace(profile))] = adapter
	}
	return &ingestHospitalExportUseCase{
		registry: registry,
		adapters: copied,
		logger:   logger,
	}
}

func (u *ingestHospitalExportUseCase) IngestFiles(
	ctx context.Context,
	profile string,
	paths []string,
) (*interfaces.IngestResult, error) {
	profile = strings.ToUpper(strings.TrimSpace(profile))
	if profile == "" {
		return nil, fmt.Errorf("profile is required")
	}
	if u.registry == nil {
		return nil, fmt.Errorf("patient registry repository is not configured")
	}
	adapter := u.adapters[profile]
	if adapter == nil {
		return nil, fmt.Errorf("unsupported hospital export profile %q", profile)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("at least one export path is required")
	}

	result := &interfaces.IngestResult{
		Profile: profile,
		Files:   len(paths),
	}
	now := time.Now().UTC()
	// Scoped to this run only (not the use case struct): a file with many
	// specimens for the same patient resolves that patient once instead of
	// once per specimen.
	patientCache := map[string]*entities.Patient{}

	for _, path := range paths {
		specimens, anomalies, err := adapter.ParseFile(path)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		result.SpecimensParsed += len(specimens)
		for _, anomaly := range anomalies {
			reason := fmt.Sprintf("%s:%d:%s", anomaly.SourceFile, anomaly.SourceRowNumber, anomaly.Reason)
			result.AnomalyReasons = append(result.AnomalyReasons, reason)
			u.logger.Warn(
				"Hospital export anomaly",
				loggerpkg.String("profile", profile),
				loggerpkg.String("source_file", anomaly.SourceFile),
				loggerpkg.Int("source_row_number", anomaly.SourceRowNumber),
				loggerpkg.String("reason", anomaly.Reason),
			)
		}

		for i := range specimens {
			specimen := specimens[i]
			if err := u.persistSpecimen(ctx, specimen, now, patientCache); err != nil {
				return nil, err
			}
			result.SpecimensSaved++
			result.ObservationsSaved += len(specimen.Observations)
		}
	}

	u.logger.Info(
		"Hospital export ingested",
		loggerpkg.String("profile", profile),
		loggerpkg.Int("files", result.Files),
		loggerpkg.Int("specimens_saved", result.SpecimensSaved),
		loggerpkg.Int("observations_saved", result.ObservationsSaved),
		loggerpkg.Int("anomalies", result.AnomalyCount()),
	)
	return result, nil
}

func (u *ingestHospitalExportUseCase) persistSpecimen(
	ctx context.Context,
	normalized domaintypes.NormalizedSpecimen,
	now time.Time,
	patientCache map[string]*entities.Patient,
) error {
	patientKey := strings.TrimSpace(normalized.ExternalPatientID)
	patient, ok := patientCache[patientKey]
	if !ok {
		var err error
		patient, err = u.registry.GetOrCreatePatient(ctx, normalized.ExternalPatientID)
		if err != nil {
			return fmt.Errorf("get or create patient %q: %w", normalized.ExternalPatientID, err)
		}
		patientCache[patientKey] = patient
	}

	specimenID := uuid.New()
	specimen := entities.NewSpecimen(entities.NewSpecimenParams{
		ID:                 specimenID,
		PatientID:          patient.ID(),
		ExternalSpecimenID: normalized.ExternalSpecimenID,
		CollectedAt:        normalized.CollectedAt,
		SourceDataset:      normalized.Provenance.SourceDataset,
		SourceFile:         normalized.Provenance.SourceFile,
		SourceRecordID:     normalized.Provenance.SourceRecordID,
		SourceRowNumber:    normalized.Provenance.SourceRowNumber,
		Now:                now,
	})

	observations := make([]*entities.LabObservation, 0, len(normalized.Observations))
	for _, observation := range normalized.Observations {
		observations = append(observations, entities.NewLabObservation(entities.NewLabObservationParams{
			ID:         uuid.New(),
			SpecimenID: specimenID,
			FieldCode:  observation.FieldCode,
			Value:      observation.Value,
			RawValue:   observation.RawValue,
			RawUnit:    observation.RawUnit,
			Censored:   observation.Censored,
			Revision:   observation.Revision,
			Now:        now,
		}))
	}

	if err := u.registry.SaveSpecimen(ctx, specimen, observations); err != nil {
		return fmt.Errorf("save specimen for patient %q: %w", normalized.ExternalPatientID, err)
	}
	return nil
}
