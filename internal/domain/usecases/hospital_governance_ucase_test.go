package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/repositories"
	domainerrors "github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases/errors"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/mocks"
)

func TestGovernedHospitalGovernanceUseCasePolicyUsesTransactionAndDerivedActor(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockHospitalGovernanceRepository(ctrl)
	tx := mocks.NewMockTransactionManager(ctrl)
	txRepos := mocks.NewMockTxRepositories(ctrl)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	now := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	u := NewHospitalGovernanceUseCase(HospitalGovernanceDeps{NodeID: "node-a", Repository: repo, Transactions: tx, Now: func() time.Time { return now }})
	tx.EXPECT().WithinTx(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, fn func(repositories.TxRepositories) error) error { return fn(txRepos) })
	txRepos.EXPECT().HospitalGovernance().Return(repo)
	gomock.InOrder(
		repo.EXPECT().Lock(ctx, "node-a").Return(nil),
		repo.EXPECT().GetCommand(ctx, "node-a", "hospital-admin", "policy-1").Return(nil, nil),
		repo.EXPECT().Get(ctx, "node-a").Return(entities.NewHospitalGovernance("node-a"), nil),
		repo.EXPECT().Save(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, s *entities.HospitalGovernance) error {
			require.EqualValues(t, 2, s.Record().Policy.Version)
			require.EqualValues(t, 15, s.Record().Policy.LocalK)
			return nil
		}),
		repo.EXPECT().AppendAudit(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, a entities.HospitalGovernanceAuditRecord) error {
			require.Equal(t, "hospital-admin", a.Actor)
			require.Equal(t, "LOCAL_POLICY_CHANGED", a.Type)
			require.Equal(t, now, a.OccurredAt)
			return nil
		}),
		repo.EXPECT().SaveCommand(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, c entities.HospitalGovernanceCommandRecord) error {
			require.Equal(t, "policy-1", c.Key)
			require.Equal(t, "hospital-admin", c.Actor)
			require.NotEmpty(t, c.Fingerprint)
			return nil
		}),
	)
	result, err := u.UpdatePolicy(ctx, "hospital-admin", "policy-1", entities.HospitalPolicyRecord{LocalK: 15, ReleaseMode: "AUTO", EnabledFieldCodes: []string{"MCV"}, FilterFieldCodes: []string{"MCV"}}, 1)
	require.NoError(t, err)
	require.EqualValues(t, 2, result.LocalRevision)
}

func TestGovernedHospitalGovernanceUseCaseCommandReplayPrecedesCurrentState(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(map[bool]string{false: "identical", true: "conflicting"}[changed], func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mocks.NewMockHospitalGovernanceRepository(ctrl)
			tx := mocks.NewMockTransactionManager(ctrl)
			txRepos := mocks.NewMockTxRepositories(ctrl)
			u := NewHospitalGovernanceUseCase(HospitalGovernanceDeps{NodeID: "node-a", Repository: repo, Transactions: tx})
			policy := entities.HospitalPolicyRecord{LocalK: 15, ReleaseMode: "AUTO", EnabledFieldCodes: []string{"MCV"}, FilterFieldCodes: []string{"MCV"}}
			fingerprint, err := hospitalPolicyCommandFingerprint(policy, 1)
			require.NoError(t, err)
			recorded := entities.NewHospitalGovernance("node-a").Record()
			tx.EXPECT().WithinTx(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, fn func(repositories.TxRepositories) error) error { return fn(txRepos) })
			txRepos.EXPECT().HospitalGovernance().Return(repo)
			repo.EXPECT().Lock(gomock.Any(), "node-a").Return(nil)
			repo.EXPECT().GetCommand(gomock.Any(), "node-a", "admin", "key").Return(&entities.HospitalGovernanceCommandRecord{Fingerprint: fingerprint, Response: recorded}, nil)
			if changed {
				policy.LocalK = 20
			}
			result, err := u.UpdatePolicy(context.Background(), "admin", "key", policy, 1)
			if changed {
				var e *domainerrors.DomainError
				require.ErrorAs(t, err, &e)
				require.Equal(t, domainerrors.ErrorTypeConflict, e.Type)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.Equal(t, recorded, *result)
			}
		})
	}
}

func TestGovernedHospitalGovernanceUseCaseStoreFailureNeverReturnsMutation(t *testing.T) {
	ctrl := gomock.NewController(t)
	tx := mocks.NewMockTransactionManager(ctrl)
	u := NewHospitalGovernanceUseCase(HospitalGovernanceDeps{NodeID: "node-a", Transactions: tx})
	cause := errors.New("private database detail")
	tx.EXPECT().WithinTx(gomock.Any(), gomock.Any()).Return(cause)
	result, err := u.UpdatePolicy(context.Background(), "admin", "key", entities.HospitalPolicyRecord{LocalK: 15, ReleaseMode: "AUTO"}, 1)
	require.Nil(t, result)
	require.ErrorIs(t, err, cause)
	var e *domainerrors.DomainError
	require.ErrorAs(t, err, &e)
	require.Equal(t, "GOVERNANCE_STORE_UNAVAILABLE", e.Code)
	require.NotContains(t, e.Message, "private")
}

func TestGovernedHospitalGovernanceUseCaseInvalidCommandNeverStartsTransaction(t *testing.T) {
	ctrl := gomock.NewController(t)
	tx := mocks.NewMockTransactionManager(ctrl)
	u := NewHospitalGovernanceUseCase(HospitalGovernanceDeps{NodeID: "node-a", Transactions: tx})
	for _, actor := range []string{"", "\nspoofed"} {
		_, err := u.UpdatePolicy(context.Background(), actor, "key", entities.HospitalPolicyRecord{LocalK: 10, ReleaseMode: "AUTO"}, 1)
		require.Error(t, err)
	}
	_, err := u.UpdatePolicy(context.Background(), "admin", "", entities.HospitalPolicyRecord{LocalK: 10, ReleaseMode: "AUTO"}, 1)
	require.Error(t, err)
}
