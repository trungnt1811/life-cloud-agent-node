package wire_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	federatedwire "github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/federated/wire"
)

func TestCompileDemoQueryTask_RoundTrip(t *testing.T) {
	task, err := federatedwire.CompileDemoQueryTaskFromFixture()
	require.NoError(t, err)
	require.NoError(t, federatedwire.ValidateQueryTaskStructureV1(task))

	require.Equal(t, uint32(1), task.GetQuerySchemaVersion())
	require.Equal(t, "2024-01-01", task.GetTimeRange().GetFrom())
	require.Equal(t, "2024-12-31", task.GetTimeRange().GetTo())
	require.Equal(t, nodev1.SpecimenPolicy_SPECIMEN_POLICY_LATEST_IN_RANGE, task.GetSpecimenPolicy())
	require.Len(t, task.GetConditions(), 2)
	require.Equal(t, "MCV", task.GetConditions()[0].GetFieldCode())
	require.Equal(t, "80", task.GetConditions()[0].GetValue().GetNumberValue())
	require.Equal(t, "MCH", task.GetConditions()[1].GetFieldCode())
	require.Equal(t, "27", task.GetConditions()[1].GetValue().GetNumberValue())
	require.Len(t, task.GetRequiredPanels(), 2)
	require.Equal(t, []string{"HB", "MCV", "MCH", "RBC"}, task.GetRequiredPanels()[0].GetFieldCodes())
	require.Equal(t, nodev1.ValueConstraint_VALUE_CONSTRAINT_EXACT, task.GetRequiredPanels()[0].GetValueConstraint())
	require.Equal(t, []string{"HBA0", "HBA2", "HBF"}, task.GetRequiredPanels()[1].GetFieldCodes())
	require.Equal(t, nodev1.ValueConstraint_VALUE_CONSTRAINT_ALLOW_CENSORED, task.GetRequiredPanels()[1].GetValueConstraint())

	payload, err := proto.Marshal(task)
	require.NoError(t, err)

	var decoded nodev1.QueryTask
	require.NoError(t, proto.Unmarshal(payload, &decoded))
	require.True(t, proto.Equal(task, &decoded))
}

func TestQueryTaskV1_RejectsNonEmptyGroupBy(t *testing.T) {
	task, err := federatedwire.CompileDemoQueryTaskFromFixture()
	require.NoError(t, err)

	task.GroupBy = []string{"HB"}
	err = federatedwire.ValidateQueryTaskStructureV1(task)
	require.Error(t, err)
	require.Contains(t, err.Error(), "group_by")
}

func TestNodeControlConnect_ServiceRegistered(t *testing.T) {
	require.NotNil(t, nodev1.NodeControl_ServiceDesc.Streams)
	require.Len(t, nodev1.NodeControl_ServiceDesc.Streams, 1)
	require.Equal(t, "Connect", nodev1.NodeControl_ServiceDesc.Streams[0].StreamName)
	require.True(t, nodev1.NodeControl_ServiceDesc.Streams[0].ServerStreams)
	require.True(t, nodev1.NodeControl_ServiceDesc.Streams[0].ClientStreams)
}
