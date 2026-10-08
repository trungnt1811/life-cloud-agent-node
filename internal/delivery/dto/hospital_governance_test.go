package dto

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
)

func TestGovernedHospitalAcceptanceDTOBindsTenantAndPermitHash(t *testing.T) {
	r := entities.NewHospitalGovernance("node-a").Record()
	r.Acceptances = []entities.HospitalAcceptanceRecord{
		{TenantID: "tenant-a", PermitID: "demo", PermitVersion: 1, PermitHash: "hash-a", Decision: "DECLINED"},
		{TenantID: "tenant-b", PermitID: "demo", PermitVersion: 1, PermitHash: "hash-b", Decision: "ACCEPTED"},
	}
	r.Snapshot.Permits = []entities.HospitalPermitRecord{
		{TenantID: "tenant-b", ID: "demo", Version: 1, PermitHash: "hash-b"},
	}
	result := NewHospitalPermitListDTO(r, 1, 20)
	require.Len(t, result.Items, 1)
	require.Equal(t, "tenant-b", result.Items[0].Acceptance.TenantID)
	require.Equal(t, "ACCEPTED", result.Items[0].Acceptance.Decision)
	require.Empty(t, NewHospitalPermitListDTO(r, 2, 20).Items)
}

func TestGovernedHospitalPolicyRequestCountEncoding(t *testing.T) {
	for _, count := range []string{"", "01", "+10", "1e1", "10.0", "-1", "9223372036854775808"} {
		_, err := (HospitalPolicyRequest{LocalK: count}).Policy()
		require.Error(t, err, count)
	}
	policy, err := (HospitalPolicyRequest{LocalK: "9223372036854775807"}).Policy()
	require.NoError(t, err)
	require.EqualValues(t, 9223372036854775807, policy.LocalK)
	request := HospitalAcceptanceRequest{ExpiresAt: time.Now().UTC(), PermitVersion: 2, Decision: "DECLINED"}
	change := request.Change("demo")
	require.Equal(t, "demo", change.PermitID)
	require.Equal(t, request.ExpiresAt, change.ExpiresAt)
}
