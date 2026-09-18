package wire_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	federatedwire "github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/federated/wire"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/mocks"
)

func TestSuppressMatchingCount(t *testing.T) {
	t.Parallel()

	matching, suppressed := federatedwire.SuppressMatchingCount(0, 5)
	require.Equal(t, uint64(0), matching)
	require.False(t, suppressed)

	matching, suppressed = federatedwire.SuppressMatchingCount(3, 5)
	require.Equal(t, uint64(0), matching)
	require.True(t, suppressed)

	matching, suppressed = federatedwire.SuppressMatchingCount(5, 5)
	require.Equal(t, uint64(5), matching)
	require.False(t, suppressed)

	// Fails closed: 0 means "not configured" (default 5), not "disabled".
	matching, suppressed = federatedwire.SuppressMatchingCount(3, 0)
	require.Equal(t, uint64(0), matching)
	require.True(t, suppressed)

	// A threshold of 1 hides nothing.
	matching, suppressed = federatedwire.SuppressMatchingCount(3, 1)
	require.Equal(t, uint64(3), matching)
	require.False(t, suppressed)
}

func TestCohortCriteriaFromQueryTaskV1_DemoFixture(t *testing.T) {
	task, err := federatedwire.CompileDemoQueryTaskFromFixture()
	require.NoError(t, err)

	criteria, err := federatedwire.CohortCriteriaFromQueryTaskV1(task)
	require.NoError(t, err)
	require.Equal(t, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), criteria.From)
	require.Equal(t, time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC), criteria.To)
	require.Len(t, criteria.Conditions, 2)
	require.Equal(t, "MCV", criteria.Conditions[0].FieldCode)
	require.Equal(t, types.ComparisonOpLT, criteria.Conditions[0].Op)
	require.Equal(t, "80", criteria.Conditions[0].NumberValue)
	require.Len(t, criteria.RequiredPanels, 2)
	require.Equal(t, types.PanelValueExact, criteria.RequiredPanels[0].Constraint)
	require.Equal(t, types.PanelValueAllowCensored, criteria.RequiredPanels[1].Constraint)
}

func TestExecuteQueryTaskV1_AppliesSuppression(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	task, err := federatedwire.CompileDemoQueryTaskFromFixture()
	require.NoError(t, err)

	repo := mocks.NewMockPatientRegistryRepository(ctrl)
	repo.EXPECT().
		CountMatchingCohort(gomock.Any(), gomock.Any()).
		Return(uint64(3), nil)

	result, err := federatedwire.ExecuteQueryTaskV1(context.Background(), task, repo, 5)
	require.NoError(t, err)
	require.Equal(t, nodev1.QueryResultStatus_QUERY_RESULT_STATUS_OK, result.GetStatus())
	require.Equal(t, uint64(0), result.GetMatchingCount())
	require.True(t, result.GetSuppressed())
	require.Equal(t, task.GetJobId(), result.GetJobId())
}

func TestExecuteQueryTaskV1_ZeroThresholdStillSuppresses(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	task, err := federatedwire.CompileDemoQueryTaskFromFixture()
	require.NoError(t, err)
	repo := mocks.NewMockPatientRegistryRepository(ctrl)
	repo.EXPECT().CountMatchingCohort(gomock.Any(), gomock.Any()).Return(uint64(2), nil)

	result, err := federatedwire.ExecuteQueryTaskV1(context.Background(), task, repo, 0)
	require.NoError(t, err)
	require.Equal(t, uint64(0), result.GetMatchingCount())
	require.True(t, result.GetSuppressed())
}

func TestExecuteQueryTaskV1_EnforcesPreconditionsWithoutTouchingRepo(t *testing.T) {
	cases := map[string]struct {
		mutate func(*nodev1.QueryTask)
		status nodev1.QueryResultStatus
	}{
		"unsupported_version": {
			func(task *nodev1.QueryTask) { task.QuerySchemaVersion = 2 },
			nodev1.QueryResultStatus_QUERY_RESULT_STATUS_UNSUPPORTED_VERSION,
		},
		"group_by": {
			func(task *nodev1.QueryTask) { task.GroupBy = []string{"HB"} },
			nodev1.QueryResultStatus_QUERY_RESULT_STATUS_REJECTED_INVALID_QUERY,
		},
		"unspecified_specimen_policy": {
			func(task *nodev1.QueryTask) {
				task.SpecimenPolicy = nodev1.SpecimenPolicy_SPECIMEN_POLICY_UNSPECIFIED
			},
			nodev1.QueryResultStatus_QUERY_RESULT_STATUS_REJECTED_INVALID_QUERY,
		},
		"non_decimal_value": {
			func(task *nodev1.QueryTask) {
				task.Conditions[0].Value = &nodev1.ConditionValue{
					Kind: &nodev1.ConditionValue_NumberValue{NumberValue: "0x10"},
				}
			},
			nodev1.QueryResultStatus_QUERY_RESULT_STATUS_REJECTED_INVALID_QUERY,
		},
		"bad_date": {
			func(task *nodev1.QueryTask) { task.TimeRange.From = "2024-1-01" },
			nodev1.QueryResultStatus_QUERY_RESULT_STATUS_REJECTED_INVALID_QUERY,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			task, err := federatedwire.CompileDemoQueryTaskFromFixture()
			require.NoError(t, err)
			tc.mutate(task)

			// No CountMatchingCohort expectation: the repo must not be reached.
			result, err := federatedwire.ExecuteQueryTaskV1(
				context.Background(), task, mocks.NewMockPatientRegistryRepository(ctrl), 5,
			)
			require.NoError(t, err)
			require.Equal(t, tc.status, result.GetStatus())
			require.NotEmpty(t, result.GetReason())
		})
	}
}

func TestExecuteQueryTaskV1_PropagatesContextErrors(t *testing.T) {
	for name, cause := range map[string]error{
		"canceled":          context.Canceled,
		"deadline_exceeded": context.DeadlineExceeded,
	} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			task, err := federatedwire.CompileDemoQueryTaskFromFixture()
			require.NoError(t, err)
			repo := mocks.NewMockPatientRegistryRepository(ctrl)
			repo.EXPECT().
				CountMatchingCohort(gomock.Any(), gomock.Any()).
				Return(uint64(0), fmt.Errorf("query: %w", cause))

			result, err := federatedwire.ExecuteQueryTaskV1(context.Background(), task, repo, 5)
			require.Nil(t, result, "an interrupted job must not become a terminal ERROR result")
			require.ErrorIs(t, err, cause)
		})
	}
}

func TestExecuteQueryTaskV1_RepoFailureIsErrorStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	task, err := federatedwire.CompileDemoQueryTaskFromFixture()
	require.NoError(t, err)

	repo := mocks.NewMockPatientRegistryRepository(ctrl)
	repo.EXPECT().
		CountMatchingCohort(gomock.Any(), gomock.Any()).
		Return(uint64(0), errors.New("db down"))

	result, err := federatedwire.ExecuteQueryTaskV1(context.Background(), task, repo, 5)
	require.NoError(t, err)
	require.Equal(t, nodev1.QueryResultStatus_QUERY_RESULT_STATUS_ERROR, result.GetStatus())
	require.Contains(t, result.GetReason(), "execution failed")
}
