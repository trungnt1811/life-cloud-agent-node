package repositories

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/models"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/testsupport/mappingtest"
)

func TestEnabledQueryFieldMapperCoversRecordAndModelFields(t *testing.T) {
	mappingtest.AssertSameNamedFieldCoverage(t, entities.EnabledQueryFieldRecord{}, models.EnabledQueryField{})
}

func TestEnabledQueryFieldMapperRoundTripsEveryField(t *testing.T) {
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)

	entity := entities.NewEnabledQueryField("hb", true, "alice", now)
	model := enabledQueryFieldModelFromEntity(entity)
	got := enabledQueryFieldEntityFromModel(model)
	require.Equal(t, entity.Record(), got.Record())
	require.Equal(
		t,
		entity.Record(),
		enabledQueryFieldEntitiesFromModels([]models.EnabledQueryField{*model})[0].Record(),
	)
}

func TestEnabledQueryFieldEntitiesFromModelsSkipsBlankFieldCode(t *testing.T) {
	got := enabledQueryFieldEntitiesFromModels([]models.EnabledQueryField{
		{FieldCode: "  ", Enabled: true},
		{FieldCode: "hb", Enabled: true},
	})
	require.Len(t, got, 1)
	require.Equal(t, "HB", got[0].FieldCode())
}
