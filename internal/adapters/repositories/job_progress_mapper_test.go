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

func TestJobProgressMapperCoversRecordAndModelFields(t *testing.T) {
	mappingtest.AssertSameNamedFieldCoverage(t, entities.JobProgressRecord{}, models.JobProgress{})
}

func TestJobProgressMapperRoundTripsEveryField(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)

	fresh := entities.NewJobProgress("job-1", "hash", now)
	require.Nil(t, jobProgressModelFromEntity(fresh).LastPatientID, "no chunk yet maps to NULL")
	require.Equal(t, fresh.Record(), jobProgressEntityFromModel(jobProgressModelFromEntity(fresh)).Record())

	advanced := entities.NewJobProgress("job-2", "hash", now)
	advanced.CompleteChunk(uuid.New(), 7, now)
	advanced.MarkDone(now)
	got := jobProgressEntityFromModel(jobProgressModelFromEntity(advanced))
	require.Equal(t, advanced.Record(), got.Record())
	require.True(t, got.Done())
}
