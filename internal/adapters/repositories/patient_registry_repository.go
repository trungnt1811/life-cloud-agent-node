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
	if ctx == nil {
		return r.db
	}
	return r.db.WithContext(ctx)
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

	existing, err := r.GetPatientByExternalID(ctx, externalPatientID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	patient := entities.NewPatient(uuid.New(), externalPatientID, time.Now().UTC())
	model := patientModelFromEntity(patient)
	if err := r.dbWithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "external_patient_id"}},
			DoNothing: true,
		}).
		Create(model).Error; err != nil {
		r.logger.Error("Failed to create patient", logger.String("external_patient_id", externalPatientID), logger.Err(err))
		return nil, err
	}

	return r.GetPatientByExternalID(ctx, externalPatientID)
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
		if err := tx.Create(specimenModel).Error; err != nil {
			r.logger.Error("Failed to create specimen", logger.String("id", specimen.ID().String()), logger.Err(err))
			return err
		}

		for _, observation := range observations {
			if observation == nil {
				continue
			}
			if observation.SpecimenID() != specimen.ID() {
				return errors.New("lab observation specimen id mismatch")
			}
			observationModel := labObservationModelFromEntity(observation)
			if err := tx.Create(observationModel).Error; err != nil {
				r.logger.Error(
					"Failed to create lab observation",
					logger.String("specimen_id", specimen.ID().String()),
					logger.String("field_code", observation.FieldCode()),
					logger.Err(err),
				)
				return err
			}
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
