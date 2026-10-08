package entities

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGovernedHospitalGovernancePinnedPermitAndPolicyHashes(t *testing.T) {
	encoded, err := os.ReadFile("testdata/hospital-governance-hash-vectors.json")
	require.NoError(t, err)
	var fixture struct {
		PermitInput struct {
			TenantID string              `json:"tenant_id"`
			ID       string              `json:"id"`
			Version  int64               `json:"version"`
			Scope    HospitalPermitScope `json:"scope"`
		} `json:"permit_input"`
		PermitHash  string `json:"permit_hash"`
		PolicyInput struct {
			NodeID            string   `json:"node_id"`
			Version           int64    `json:"version"`
			LocalK            uint64   `json:"local_k"`
			ReleaseMode       string   `json:"release_mode"`
			EnabledFieldCodes []string `json:"enabled_field_codes"`
			FilterFieldCodes  []string `json:"filter_field_codes"`
			PanelFieldCodes   []string `json:"panel_field_codes"`
		} `json:"policy_input"`
		PolicyHash string `json:"policy_hash"`
	}
	require.NoError(t, json.Unmarshal(encoded, &fixture))
	p := fixture.PermitInput
	permit := HospitalPermitRecord{TenantID: p.TenantID, ID: p.ID, Version: p.Version, HospitalPermitScope: p.Scope}
	hash, err := permit.Hash()
	require.NoError(t, err)
	require.Equal(t, fixture.PermitHash, hash)
	policy := fixture.PolicyInput
	state := NewHospitalGovernance(policy.NodeID)
	require.NoError(t, state.UpdatePolicy(HospitalPolicyRecord{
		LocalK: policy.LocalK, ReleaseMode: policy.ReleaseMode,
		EnabledFieldCodes: policy.EnabledFieldCodes, FilterFieldCodes: policy.FilterFieldCodes, PanelFieldCodes: policy.PanelFieldCodes,
	}, 1))
	require.Equal(t, fixture.PolicyHash, state.Record().Policy.PolicyHash)
	require.Equal(t, policy.Version, state.Record().Policy.Version)
}
