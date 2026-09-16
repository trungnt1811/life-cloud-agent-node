package entities_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
)

func TestNewSpecimen_CollectedAtKeepsLocalCalendarDate(t *testing.T) {
	// 06:30 local time in a positive-offset timezone (Vietnam, UTC+7) is
	// still 23:30 the PREVIOUS day in UTC. Converting to UTC before taking
	// the calendar date would silently record this specimen a day early.
	loc := time.FixedZone("UTC+7", 7*60*60)
	collectedAt := time.Date(2024, 6, 15, 6, 30, 0, 0, loc)

	specimen := entities.NewSpecimen(entities.NewSpecimenParams{
		PatientID:      uuid.New(),
		CollectedAt:    collectedAt,
		SourceDataset:  "VN_A",
		SourceFile:     "cbc.csv",
		SourceRecordID: "row-1",
	})

	require.Equal(t, time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC), specimen.CollectedAt())
}
