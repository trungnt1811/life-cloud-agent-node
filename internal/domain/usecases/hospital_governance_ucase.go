package usecases

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/repositories"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
	domainerrors "github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases/errors"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/usecases/interfaces"
)

type HospitalGovernanceDeps struct {
	NodeID       string
	Repository   repositories.HospitalGovernanceRepository
	Transactions repositories.TransactionManager
	Now          func() time.Time
}

type hospitalGovernanceUseCase struct{ deps HospitalGovernanceDeps }

func NewHospitalGovernanceUseCase(deps HospitalGovernanceDeps) interfaces.HospitalGovernanceUseCase {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	return &hospitalGovernanceUseCase{deps: deps}
}

func (u *hospitalGovernanceUseCase) Read(ctx context.Context) (*entities.HospitalGovernanceRecord, error) {
	if u.deps.Repository == nil || !validHospitalCommandPart(u.deps.NodeID, 128) {
		return nil, hospitalGovernanceError(errors.New("hospital governance repository or node identity missing"))
	}
	state, err := u.load(ctx, u.deps.Repository)
	if err != nil {
		return nil, hospitalGovernanceError(err)
	}
	record := state.Record()
	return &record, nil
}

func (u *hospitalGovernanceUseCase) UpdatePolicy(ctx context.Context, actor, key string, policy entities.HospitalPolicyRecord, expected int64) (*entities.HospitalGovernanceRecord, error) {
	if expected < 1 || expected == math.MaxInt64 {
		return nil, hospitalGovernanceError(entities.ErrHospitalGovernanceInvalid)
	}
	fingerprint, err := hospitalPolicyCommandFingerprint(policy, expected)
	if err != nil {
		return nil, hospitalGovernanceError(err)
	}
	return u.command(ctx, actor, key, fingerprint, "LOCAL_POLICY_CHANGED", "", 0, "", "", func(state *entities.HospitalGovernance, _ time.Time) error {
		return state.UpdatePolicy(policy, expected)
	})
}

func hospitalPolicyCommandFingerprint(policy entities.HospitalPolicyRecord, expected int64) (string, error) {
	normalized := entities.NewHospitalGovernance("fingerprint-only")
	if err := normalized.UpdatePolicy(policy, 1); err != nil {
		return "", err
	}
	policy = normalized.Record().Policy
	// Server-owned version/hash are neither accepted nor fingerprinted as caller intent.
	return types.GovernanceDigest("local-policy-command/v1", map[string]any{
		"expected_revision": expected, "local_k": policy.LocalK, "release_mode": policy.ReleaseMode,
		"enabled_field_codes": policy.EnabledFieldCodes, "filter_field_codes": policy.FilterFieldCodes, "panel_field_codes": policy.PanelFieldCodes,
	})
}

func (u *hospitalGovernanceUseCase) ChangeAcceptance(ctx context.Context, actor, key string, change entities.HospitalAcceptanceChange) (*entities.HospitalGovernanceRecord, error) {
	change, err := entities.NormalizeHospitalAcceptanceChange(change)
	if err != nil {
		return nil, hospitalGovernanceError(err)
	}
	fingerprint, err := types.GovernanceDigest("local-acceptance-command/v1", map[string]any{
		"permit_id": change.PermitID, "permit_version": change.PermitVersion, "permit_hash": change.PermitHash,
		"decision": change.Decision, "allowed_field_codes": change.AllowedFieldCodes, "expires_at": change.ExpiresAt.UTC(), "expected_revision": change.ExpectedRevision,
	})
	if err != nil {
		return nil, hospitalGovernanceError(err)
	}
	return u.command(ctx, actor, key, fingerprint, "PERMIT_ACCEPTANCE_CHANGED", change.PermitID, change.PermitVersion, "", "", func(state *entities.HospitalGovernance, now time.Time) error {
		return state.Accept(change, now)
	})
}

func (u *hospitalGovernanceUseCase) command(ctx context.Context, actor, key, fingerprint, event, permitID string, permitVersion int64, epoch, queryID string, apply func(*entities.HospitalGovernance, time.Time) error) (*entities.HospitalGovernanceRecord, error) {
	if !validHospitalCommandPart(actor, 128) || !validHospitalCommandPart(key, 255) {
		return nil, domainerrors.NewValidationError("INVALID_GOVERNANCE_COMMAND", "authenticated actor and Idempotency-Key are required")
	}
	var result *entities.HospitalGovernanceRecord
	err := u.transaction(ctx, func(repo repositories.HospitalGovernanceRepository) error {
		var state *entities.HospitalGovernance
		if epoch != "" {
			var err error
			state, err = u.load(ctx, repo)
			if err != nil {
				return err
			}
			if state.Record().Snapshot.SessionEpoch != epoch {
				return entities.ErrHospitalGovernanceConflict
			}
		}
		command, err := repo.GetCommand(ctx, u.deps.NodeID, actor, key)
		if err != nil {
			return err
		}
		if command != nil {
			if command.Fingerprint != fingerprint {
				return entities.ErrHospitalGovernanceConflict
			}
			record := entities.NewHospitalGovernanceFromRecord(command.Response).Record()
			result = &record
			return nil
		}
		if state == nil {
			state, err = u.load(ctx, repo)
			if err != nil {
				return err
			}
		}
		now := u.deps.Now().UTC().Truncate(time.Microsecond)
		if err := apply(state, now); err != nil {
			return err
		}
		refs := hospitalAuditReferences{queryID: queryID}
		if actor == "control-plane" {
			refs.commandID = key
		}
		if err := u.saveReferences(ctx, repo, state, event, actor, permitID, permitVersion, now, refs); err != nil {
			return err
		}
		record := state.Record()
		if err := repo.SaveCommand(ctx, entities.HospitalGovernanceCommandRecord{
			NodeID: u.deps.NodeID, Actor: actor, Key: key, Fingerprint: fingerprint, Response: record, CreatedAt: now,
		}); err != nil {
			return err
		}
		result = &record
		return nil
	})
	if err != nil {
		return nil, hospitalGovernanceError(err)
	}
	return result, nil
}

func (u *hospitalGovernanceUseCase) ApplySnapshot(ctx context.Context, snapshot entities.HospitalSnapshot) (*entities.HospitalGovernanceRecord, error) {
	var result *entities.HospitalGovernanceRecord
	err := u.transaction(ctx, func(repo repositories.HospitalGovernanceRepository) error {
		state, err := u.load(ctx, repo)
		if err != nil {
			return err
		}
		now := u.deps.Now().UTC().Truncate(time.Microsecond)
		before := state.Record()
		if err := state.ApplySnapshot(snapshot, now); err != nil {
			return err
		}
		if !reflect.DeepEqual(before, state.Record()) {
			if err := u.save(ctx, repo, state, "GOVERNANCE_SNAPSHOT_APPLIED", "control-plane", "", 0, now); err != nil {
				return err
			}
		}
		record := state.Record()
		result = &record
		return nil
	})
	if err != nil {
		return nil, hospitalGovernanceError(err)
	}
	return result, nil
}

func (u *hospitalGovernanceUseCase) Acknowledge(ctx context.Context, epoch string, snapshot, local int64) error {
	return hospitalGovernanceError(u.transaction(ctx, func(repo repositories.HospitalGovernanceRepository) error {
		state, err := u.load(ctx, repo)
		if err != nil {
			return err
		}
		prior := state.Record().AcknowledgedRevision
		if err := state.Acknowledge(epoch, snapshot, local); err != nil {
			return err
		}
		if prior == state.Record().AcknowledgedRevision {
			return nil
		}
		return u.save(ctx, repo, state, "GOVERNANCE_ACKNOWLEDGED", "control-plane", "", 0, u.deps.Now().UTC().Truncate(time.Microsecond))
	}))
}

func (u *hospitalGovernanceUseCase) FenceConnection(ctx context.Context) error {
	return hospitalGovernanceError(u.transaction(ctx, func(repo repositories.HospitalGovernanceRepository) error {
		state, err := u.load(ctx, repo)
		if err != nil {
			return err
		}
		before := state.Record()
		state.FenceConnection()
		if reflect.DeepEqual(before, state.Record()) {
			return nil
		}
		return u.save(ctx, repo, state, "GOVERNANCE_SESSION_FENCED", "node-agent", "", 0, u.deps.Now().UTC().Truncate(time.Microsecond))
	}))
}

func (u *hospitalGovernanceUseCase) transaction(ctx context.Context, fn func(repositories.HospitalGovernanceRepository) error) error {
	if u.deps.Transactions == nil || !validHospitalCommandPart(u.deps.NodeID, 128) {
		return errors.New("hospital governance transaction manager or node identity missing")
	}
	return u.deps.Transactions.WithinTx(ctx, func(repos repositories.TxRepositories) error {
		repo := repos.HospitalGovernance()
		if repo == nil {
			return errors.New("transaction-bound hospital governance repository missing")
		}
		if err := repo.Lock(ctx, u.deps.NodeID); err != nil {
			return err
		}
		return fn(repo)
	})
}

func (u *hospitalGovernanceUseCase) load(ctx context.Context, repo repositories.HospitalGovernanceRepository) (*entities.HospitalGovernance, error) {
	state, err := repo.Get(ctx, u.deps.NodeID)
	if err == nil && state == nil {
		state = entities.NewHospitalGovernance(u.deps.NodeID)
	}
	return state, err
}

func (u *hospitalGovernanceUseCase) save(ctx context.Context, repo repositories.HospitalGovernanceRepository, state *entities.HospitalGovernance, event, actor, permitID string, version int64, now time.Time) error {
	return u.saveReferences(ctx, repo, state, event, actor, permitID, version, now, hospitalAuditReferences{})
}

type hospitalAuditReferences struct{ queryID, commandID string }

func (u *hospitalGovernanceUseCase) saveReferences(ctx context.Context, repo repositories.HospitalGovernanceRepository, state *entities.HospitalGovernance, event, actor, permitID string, version int64, now time.Time, refs hospitalAuditReferences) error {
	if err := repo.Save(ctx, state); err != nil {
		return err
	}
	r := state.Record()
	audit := entities.HospitalGovernanceAuditRecord{
		EventID: uuid.NewString(), NodeID: u.deps.NodeID, Type: event, Actor: actor,
		LocalRevision: r.LocalRevision, SnapshotRevision: r.Snapshot.Revision, PermitID: permitID, PermitVersion: version,
		PolicyHash: r.Policy.PolicyHash, OccurredAt: now,
	}
	if refs.queryID != "" {
		audit.QueryID = &refs.queryID
	}
	if refs.commandID != "" {
		audit.CommandID = &refs.commandID
	}
	return repo.AppendAudit(ctx, audit)
}

func validHospitalCommandPart(value string, max int) bool {
	if value == "" || strings.TrimSpace(value) != value || len(value) > max {
		return false
	}
	for _, c := range value {
		if c < 32 || c == 127 {
			return false
		}
	}
	return true
}

func hospitalGovernanceError(err error) error {
	if err == nil {
		return nil
	}
	var e *domainerrors.DomainError
	if errors.As(err, &e) {
		return err
	}
	if errors.Is(err, entities.ErrHospitalGovernanceConflict) {
		return domainerrors.NewConflictError("GOVERNANCE_REVISION_CONFLICT", "governance command conflicts with the recorded revision or intent")
	}
	if errors.Is(err, entities.ErrHospitalGovernanceInvalid) {
		return domainerrors.NewValidationError("INVALID_GOVERNANCE_SCOPE", "governance scope is invalid or no longer authorized")
	}
	return &domainerrors.DomainError{Type: domainerrors.ErrorTypeUnavailable, Code: "GOVERNANCE_STORE_UNAVAILABLE", Message: "hospital governance store is unavailable", Cause: err}
}
