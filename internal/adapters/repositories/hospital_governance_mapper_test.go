package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/repositories/models"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/testsupport/mappingtest"
)

func TestGovernedHospitalGovernanceAuditMapperCoversEveryField(t *testing.T) {
	mappingtest.AssertSameNamedFieldCoverage(t, entities.HospitalGovernanceAuditRecord{}, models.HospitalGovernanceAudit{})
	db := openHospitalGovernanceTestDB(t)
	repo := NewHospitalGovernanceRepository(db, nil)
	event := entities.HospitalGovernanceAuditRecord{
		EventID: uuid.NewString(), NodeID: "node-a", Type: "PERMIT_ACCEPTANCE_CHANGED", Actor: "hospital-admin", LocalRevision: 4, SnapshotRevision: 2,
		PermitID: "demo", PermitVersion: 2, PolicyHash: "sha256:policy-reference", OccurredAt: time.Now().UTC().Truncate(time.Microsecond),
	}
	require.NoError(t, repo.AppendAudit(context.Background(), event))
	var model models.HospitalGovernanceAudit
	require.NoError(t, db.First(&model, "event_id = ?", event.EventID).Error)
	model.OccurredAt = model.OccurredAt.UTC()
	require.Equal(t, models.HospitalGovernanceAudit{
		EventID: event.EventID, NodeID: event.NodeID, Type: event.Type, Actor: event.Actor, LocalRevision: event.LocalRevision, SnapshotRevision: event.SnapshotRevision,
		PermitID: event.PermitID, PermitVersion: event.PermitVersion, PolicyHash: event.PolicyHash, OccurredAt: event.OccurredAt,
	}, model)
}
