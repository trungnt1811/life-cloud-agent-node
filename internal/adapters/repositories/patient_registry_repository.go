package repositories

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/models"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/repohelpers"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/repositories"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
)

type patientRegistryRepository struct {
	db     *gorm.DB
	logger logger.Logger
}

// NewPatientRegistryRepository creates a PatientRegistryRepository backed by GORM.
func NewPatientRegistryRepository(db *gorm.DB, logger logger.Logger) repositories.PatientRegistryRepository {
	return &patientRegistryRepository{
		db:     db,
		logger: logger,
	}
}

func (r *patientRegistryRepository) dbWithContext(ctx context.Context) *gorm.DB {
	return repohelpers.DBWithContext(ctx, r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *patientRegistryRepository) WithTx(tx *gorm.DB) repositories.PatientRegistryRepository {
	if tx == nil {
		return r
	}
	return &patientRegistryRepository{
		db:     tx,
		logger: r.logger,
	}
}

func (r *patientRegistryRepository) GetPatientByID(ctx context.Context, id uuid.UUID) (*entities.Patient, error) {
	if id == uuid.Nil {
		return nil, nil
	}
	var model models.Patient
	if err := r.dbWithContext(ctx).First(&model, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		r.logger.Error("Failed to get patient by ID", logger.String("id", id.String()), logger.Err(err))
		return nil, err
	}
	return patientEntityFromModel(&model), nil
}

func (r *patientRegistryRepository) GetPatientByExternalID(ctx context.Context, externalPatientID string) (*entities.Patient, error) {
	externalPatientID = strings.TrimSpace(externalPatientID)
	if externalPatientID == "" {
		return nil, nil
	}
	var model models.Patient
	if err := r.dbWithContext(ctx).First(&model, "external_patient_id = ?", externalPatientID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		r.logger.Error("Failed to get patient by external ID", logger.String("external_patient_id", externalPatientID), logger.Err(err))
		return nil, err
	}
	return patientEntityFromModel(&model), nil
}

func (r *patientRegistryRepository) GetOrCreatePatient(ctx context.Context, externalPatientID string) (*entities.Patient, error) {
	externalPatientID = strings.TrimSpace(externalPatientID)
	if externalPatientID == "" {
		return nil, errors.New("external patient id is required")
	}

	patient := entities.NewPatient(uuid.New(), externalPatientID, time.Now().UTC())
	model := patientModelFromEntity(patient)
	// DoUpdates with a no-op self-assignment (instead of DoNothing) makes
	// Postgres RETURNING the canonical row in every case - the pre-existing
	// one on conflict, the newly inserted one otherwise - in one round trip,
	// instead of a SELECT-then-maybe-INSERT-then-SELECT sequence.
	if err := r.dbWithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "external_patient_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"external_patient_id"}),
		}).
		Create(model).Error; err != nil {
		r.logger.Error("Failed to upsert patient", logger.String("external_patient_id", externalPatientID), logger.Err(err))
		return nil, err
	}

	return patientEntityFromModel(model), nil
}

func (r *patientRegistryRepository) GetSpecimenByID(ctx context.Context, id uuid.UUID) (*entities.Specimen, error) {
	if id == uuid.Nil {
		return nil, nil
	}
	var model models.Specimen
	if err := r.dbWithContext(ctx).First(&model, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		r.logger.Error("Failed to get specimen by ID", logger.String("id", id.String()), logger.Err(err))
		return nil, err
	}
	return specimenEntityFromModel(&model), nil
}

func (r *patientRegistryRepository) SaveSpecimen(
	ctx context.Context,
	specimen *entities.Specimen,
	observations []*entities.LabObservation,
) error {
	if specimen == nil {
		return errors.New("specimen is required")
	}
	if specimen.PatientID() == uuid.Nil {
		return errors.New("specimen patient id is required")
	}

	return r.dbWithContext(ctx).Transaction(func(tx *gorm.DB) error {
		specimenModel := specimenModelFromEntity(specimen)
		// DoNothing on the specimen's real-world identity (dataset +
		// patient + the hospital's own specimen ID - not file/row
		// provenance) makes a retried or re-exported ingestion batch a
		// safe no-op for the specimen row instead of a duplicate insert.
		// Unlike file/row provenance, this identity is stable across a
		// corrected re-export under a new filename or shifted row numbers.
		if err := tx.
			Clauses(clause.OnConflict{
				Columns: []clause.Column{
					{Name: "source_dataset"}, {Name: "patient_id"}, {Name: "external_specimen_id"},
				},
				DoNothing: true,
			}).
			Create(specimenModel).Error; err != nil {
			r.logger.Error("Failed to create specimen", logger.String("id", specimen.ID().String()), logger.Err(err))
			return err
		}

		// The conflict clause may have skipped the insert if this specimen
		// was already ingested under a different, earlier-assigned ID; read
		// back the canonical row by its natural key so observations attach
		// to the row that actually exists rather than the one just attempted.
		var canonical models.Specimen
		if err := tx.
			Where(
				"source_dataset = ? AND patient_id = ? AND external_specimen_id = ?",
				specimenModel.SourceDataset, specimenModel.PatientID, specimenModel.ExternalSpecimenID,
			).
			First(&canonical).Error; err != nil {
			r.logger.Error("Failed to read back specimen", logger.String("id", specimen.ID().String()), logger.Err(err))
			return err
		}

		observationModels := make([]*models.LabObservation, 0, len(observations))
		for _, observation := range observations {
			if observation == nil {
				continue
			}
			if observation.SpecimenID() != specimen.ID() {
				return errors.New("lab observation specimen id mismatch")
			}
			observationModel := labObservationModelFromEntity(observation)
			observationModel.SpecimenID = canonical.ID
			observationModels = append(observationModels, observationModel)
		}
		if len(observationModels) == 0 {
			return nil
		}

		// One batched insert instead of one round trip per observation. A
		// re-ingested (specimen_id, field_code) overwrites value/censored/
		// raw_value/raw_unit/revision only when the incoming revision is >=
		// the stored one - "highest revision wins" regardless of which
		// ingest run happens to execute last. A later hospital export
		// correcting an earlier result (e.g. MCV 70 -> 85) must win; a
		// stale re-ingest of an older export must not clobber a correction.
		if err := tx.
			Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "specimen_id"}, {Name: "field_code"}},
				DoUpdates: clause.AssignmentColumns([]string{"value", "censored", "raw_value", "raw_unit", "revision"}),
				Where: clause.Where{
					Exprs: []clause.Expression{
						gorm.Expr("lab_observations.revision <= EXCLUDED.revision"),
					},
				},
			}).
			Create(&observationModels).Error; err != nil {
			r.logger.Error(
				"Failed to create lab observations",
				logger.String("specimen_id", canonical.ID.String()),
				logger.Err(err),
			)
			return err
		}
		return nil
	})
}

func (r *patientRegistryRepository) ListObservationsBySpecimenID(
	ctx context.Context,
	specimenID uuid.UUID,
) ([]*entities.LabObservation, error) {
	if specimenID == uuid.Nil {
		return []*entities.LabObservation{}, nil
	}

	var modelsList []models.LabObservation
	if err := r.dbWithContext(ctx).
		Where("specimen_id = ?", specimenID).
		Order("field_code ASC, id ASC").
		Find(&modelsList).Error; err != nil {
		r.logger.Error("Failed to list lab observations", logger.String("specimen_id", specimenID.String()), logger.Err(err))
		return nil, err
	}
	return labObservationEntitiesFromModels(modelsList), nil
}

func (r *patientRegistryRepository) CountPatients(ctx context.Context) (int, error) {
	var count int64
	if err := r.dbWithContext(ctx).Model(&models.Patient{}).Count(&count).Error; err != nil {
		r.logger.Error("Failed to count patients", logger.Err(err))
		return 0, err
	}
	return int(count), nil
}

func (r *patientRegistryRepository) CountSpecimens(ctx context.Context) (int, error) {
	var count int64
	if err := r.dbWithContext(ctx).Model(&models.Specimen{}).Count(&count).Error; err != nil {
		r.logger.Error("Failed to count specimens", logger.Err(err))
		return 0, err
	}
	return int(count), nil
}

func (r *patientRegistryRepository) CountLabObservations(ctx context.Context) (int, error) {
	var count int64
	if err := r.dbWithContext(ctx).Model(&models.LabObservation{}).Count(&count).Error; err != nil {
		r.logger.Error("Failed to count lab observations", logger.Err(err))
		return 0, err
	}
	return int(count), nil
}
