package usecases

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
)

func TestGovernedHospitalGovernancePolicyCommandCanonicalizesScope(t *testing.T) {
	p := entities.HospitalPolicyRecord{LocalK: 15, ReleaseMode: "AUTO", EnabledFieldCodes: []string{"mcv", "MCH", "MCV"}, FilterFieldCodes: []string{" mcv "}}
	first, err := hospitalPolicyCommandFingerprint(p, 1)
	require.NoError(t, err)
	p.EnabledFieldCodes, p.FilterFieldCodes, p.PanelFieldCodes = []string{"MCH", "MCV"}, []string{"MCV"}, []string{}
	second, err := hospitalPolicyCommandFingerprint(p, 1)
	require.NoError(t, err)
	require.Equal(t, first, second)
}
