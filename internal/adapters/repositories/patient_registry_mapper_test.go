package repositories

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/models"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/testsupport/mappingtest"
)

func TestPatientRegistryMapperCoversRecordAndModelFields(t *testing.T) {
	mappingtest.AssertSameNamedFieldCoverage(t, entities.PatientRecord{}, models.Patient{})
	mappingtest.AssertSameNamedFieldCoverage(t, entities.SpecimenRecord{}, models.Specimen{})
	mappingtest.AssertSameNamedFieldCoverage(t, entities.LabObservationRecord{}, models.LabObservation{})
}

func TestPatientRegistryMapperRoundTripsEveryField(t *testing.T) {
	now := time.Date(2026, 9, 16, 10, 30, 0, 123, time.UTC)
	collectedAt := time.Date(2024, 6, 15, 18, 45, 0, 0, time.UTC)

	patient := entities.NewPatient(uuid.New(), "EXT-001", now)
	patientModel := patientModelFromEntity(patient)
	gotPatient := patientEntityFromModel(patientModel)
	require.Equal(t, patient.Record(), gotPatient.Record())
	require.Nil(t, patientModelFromEntity(nil))
	require.Nil(t, patientEntityFromModel(nil))

	specimen := entities.NewSpecimen(
		uuid.New(),
		patient.ID(),
		collectedAt,
		"VN_A",
		"raw/cbc.csv",
		"row-12",
		12,
		now,
	)
	require.Equal(t, time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC), specimen.CollectedAt())
	specimenModel := specimenModelFromEntity(specimen)
	gotSpecimen := specimenEntityFromModel(specimenModel)
	require.Equal(t, specimen.Record(), gotSpecimen.Record())
	require.Nil(t, specimenModelFromEntity(nil))
	require.Nil(t, specimenEntityFromModel(nil))

	observation := entities.NewLabObservation(
		uuid.New(),
		specimen.ID(),
		"MCV",
		"79.5",
		"<0.1",
		"fL",
		true,
		now,
	)
	observationModel := labObservationModelFromEntity(observation)
	gotObservation := labObservationEntityFromModel(observationModel)
	require.Equal(t, observation.Record(), gotObservation.Record())
	require.Nil(t, labObservationModelFromEntity(nil))
	require.Nil(t, labObservationEntityFromModel(nil))
	require.Equal(
		t,
		observation.Record(),
		labObservationEntitiesFromModels([]models.LabObservation{*observationModel})[0].Record(),
	)
}
