package repositories

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/go-backend-template/internal/adapters/repositories/models"
	"github.com/lifenetwork-ai/go-backend-template/internal/domain/entities"
	"github.com/lifenetwork-ai/go-backend-template/internal/testsupport/mappingtest"
)

func TestExampleMapperCoversRecordAndModelFields(t *testing.T) {
	mappingtest.AssertSameNamedFieldCoverage(t, entities.ExampleRecord{}, models.Example{})
}

func TestExampleMapperRoundTripsEveryField(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 30, 0, 123, time.UTC)
	entity := entities.NewExampleEntity(uuid.New(), "Alice", "first example", now)

	model := exampleModelFromEntity(entity)
	got := exampleEntityFromModel(model)

	require.Equal(t, entity.Record(), got.Record())
	require.Nil(t, exampleModelFromEntity(nil))
	require.Nil(t, exampleEntityFromModel(nil))
	require.Equal(t, entity.Record(), exampleEntitiesFromModels([]models.Example{*model})[0].Record())
}
