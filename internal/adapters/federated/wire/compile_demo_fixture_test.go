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

func TestCompileDemoQueryTask_ExportedAPI(t *testing.T) {
	task := federatedwire.CompileDemoQueryTask(federatedwire.DemoQueryDefinition{
		DateStart:    "2024-01-01",
		DateEnd:      "2024-12-31",
		CBCRequired:  []string{"HB"},
		HPLCRequired: []string{"HBF"},
	})
	require.Equal(t, "2024-01-01", task.GetTimeRange().GetFrom())
	require.Equal(t, []string{"HB"}, task.GetRequiredPanels()[0].GetFieldCodes())
}

func TestQueryTaskV1_RejectsStructuralViolations(t *testing.T) {
	t.Run("non_empty_group_by", func(t *testing.T) {
		task, err := federatedwire.CompileDemoQueryTaskFromFixture()
		require.NoError(t, err)
		task.GroupBy = []string{"HB"}
		err = federatedwire.ValidateQueryTaskStructureV1(task)
		require.Error(t, err)
		require.Contains(t, err.Error(), "group_by")
	})

	t.Run("unsupported_schema_version", func(t *testing.T) {
		task, err := federatedwire.CompileDemoQueryTaskFromFixture()
		require.NoError(t, err)
		task.QuerySchemaVersion = 2
		err = federatedwire.ValidateQueryTaskStructureV1(task)
		require.Error(t, err)
		require.Contains(t, err.Error(), "query_schema_version")
	})

	t.Run("unspecified_specimen_policy", func(t *testing.T) {
		task, err := federatedwire.CompileDemoQueryTaskFromFixture()
		require.NoError(t, err)
		task.SpecimenPolicy = nodev1.SpecimenPolicy_SPECIMEN_POLICY_UNSPECIFIED
		err = federatedwire.ValidateQueryTaskStructureV1(task)
		require.Error(t, err)
		require.Contains(t, err.Error(), "specimen_policy")
	})

	t.Run("unspecified_operator", func(t *testing.T) {
		task, err := federatedwire.CompileDemoQueryTaskFromFixture()
		require.NoError(t, err)
		task.Conditions[0].Op = nodev1.ComparisonOperator_COMPARISON_OPERATOR_UNSPECIFIED
		err = federatedwire.ValidateQueryTaskStructureV1(task)
		require.Error(t, err)
		require.Contains(t, err.Error(), "op")
	})

	t.Run("unspecified_value_constraint", func(t *testing.T) {
		task, err := federatedwire.CompileDemoQueryTaskFromFixture()
		require.NoError(t, err)
		task.RequiredPanels[0].ValueConstraint = nodev1.ValueConstraint_VALUE_CONSTRAINT_UNSPECIFIED
		err = federatedwire.ValidateQueryTaskStructureV1(task)
		require.Error(t, err)
		require.Contains(t, err.Error(), "value_constraint")
	})

	t.Run("missing_time_range", func(t *testing.T) {
		task, err := federatedwire.CompileDemoQueryTaskFromFixture()
		require.NoError(t, err)
		task.TimeRange = nil
		err = federatedwire.ValidateQueryTaskStructureV1(task)
		require.Error(t, err)
		require.Contains(t, err.Error(), "time_range")
	})

	t.Run("missing_time_range_bound", func(t *testing.T) {
		task, err := federatedwire.CompileDemoQueryTaskFromFixture()
		require.NoError(t, err)
		task.TimeRange.To = ""
		err = federatedwire.ValidateQueryTaskStructureV1(task)
		require.Error(t, err)
		require.Contains(t, err.Error(), "time_range.to")
	})
}

func TestNodeControlConnect_ServiceRegistered(t *testing.T) {
	require.NotNil(t, nodev1.NodeControl_ServiceDesc.Streams)
	require.Len(t, nodev1.NodeControl_ServiceDesc.Streams, 1)
	require.Equal(t, "Connect", nodev1.NodeControl_ServiceDesc.Streams[0].StreamName)
	require.True(t, nodev1.NodeControl_ServiceDesc.Streams[0].ServerStreams)
	require.True(t, nodev1.NodeControl_ServiceDesc.Streams[0].ClientStreams)
}
