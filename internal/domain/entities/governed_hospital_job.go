package entities

import (
	"errors"
	"math"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
)

var ErrGovernedHospitalJobInvalid = errors.New("invalid or unauthorized governed hospital job")

const (
	GovernedHospitalExecuting       = "EXECUTING"
	GovernedHospitalReady           = "READY_TO_RELEASE"
	GovernedHospitalWaitingApproval = "WAITING_APPROVAL"
	GovernedHospitalPrepared        = "PREPARED"
	GovernedHospitalSentUnconfirmed = "SENT_UNCONFIRMED"
	GovernedHospitalRejected        = "REJECTED"
	GovernedHospitalCommitted       = "COMMITTED"
	GovernedHospitalReceived        = "RECEIVED"
	GovernedHospitalPolicyDenied    = "POLICY_DENIED"
	GovernedHospitalSucceeded       = "SUCCEEDED"
	GovernedHospitalDeclined        = "DECLINED"
	GovernedHospitalCanceled        = "CANCELED"
	GovernedHospitalFailed          = "FAILED"
	GovernedHospitalTimedOut        = "TIMED_OUT"
	GovernedHospitalApprovalExpired = "APPROVAL_EXPIRED"
)

type GovernedHospitalTask struct {
	JobID, QueryID, NodeID, TenantID, PrincipalID, Purpose string
	Criteria                                               types.CohortCriteria
	DefinitionHash, AuthorizationSnapshotHash              string
	Permit                                                 HospitalPermitRecord
	LocalPolicyVersion                                     int64
	LocalPolicyHash                                        string
	AcceptanceRevision                                     int64
	ReleaseMode                                            string
	EffectiveK                                             uint64
	DispatchDeadline, QueryDeadline                        time.Time
	SessionEpoch                                           string
}

type GovernedHospitalBinding struct {
	TenantID, NodeID, QueryID, JobID, EventID        string
	ProtectedPayloadDigest, DefinitionHash, PermitID string
	PermitVersion                                    int64
	PermitHash                                       string
	LocalPolicyVersion                               int64
	LocalPolicyHash                                  string
	AcceptanceRevision                               int64
	AuthorizationSnapshotHash, SessionEpoch          string
}

type GovernedHospitalGrant struct {
	ID                   string
	Binding              GovernedHospitalBinding
	GrantedAt, ExpiresAt time.Time
}

type GovernedHospitalReceipt struct {
	ID, JobID, EventID, ProtectedPayloadDigest, Outcome, Reason string
	CommittedAt                                                 time.Time
}

type GovernedHospitalDenial struct {
	Binding GovernedHospitalBinding
	Reason  string
}

type GovernedHospitalJobRecord struct {
	Task                                  GovernedHospitalTask
	Fingerprint                           string
	Revision                              int64
	Phase, Reason, DeliveryState          string
	ExecutionStartedAt, ExecutionDeadline time.Time
	ExecutionDurationMilliseconds         uint64
	ApprovalDeadline                      time.Time
	Approval                              *GovernedHospitalApproval
	ProtectedCount                        *types.GovernedProtectedCount
	Binding                               GovernedHospitalBinding
	Grant                                 *GovernedHospitalGrant
	Receipt                               *GovernedHospitalReceipt
	Denial                                *GovernedHospitalDenial
	Stages                                []GovernedHospitalStage
}

type GovernedHospitalJob struct{ record GovernedHospitalJobRecord }

func NewGovernedHospitalJob(task GovernedHospitalTask, state *HospitalGovernance, now time.Time) (*GovernedHospitalJob, error) {
	if err := task.Validate(state, now, true); err != nil {
		return nil, err
	}
	fingerprint, err := task.Fingerprint()
	if err != nil {
		return nil, err
	}
	job := &GovernedHospitalJob{record: GovernedHospitalJobRecord{Task: task, Fingerprint: fingerprint, Revision: 1, Phase: GovernedHospitalExecuting, ExecutionStartedAt: now.UTC(), ExecutionDeadline: minHospitalDeadline(now.Add(30*time.Second), task.QueryDeadline)}}
	job.record.Stages = []GovernedHospitalStage{initialGovernedHospitalStage(task, now)}
	job.record = cloneGovernedHospitalJob(job.record)
	return job, nil
}

func NewGovernedHospitalJobFromRecord(record GovernedHospitalJobRecord) (*GovernedHospitalJob, error) {
	fingerprint, err := record.Task.Fingerprint()
	if err != nil || fingerprint != record.Fingerprint || record.Revision < 1 || !governedHospitalPhase(record.Phase) || !validGovernedHospitalExecutionClock(record) {
		return nil, ErrGovernedHospitalJobInvalid
	}
	if err := validateGovernedHospitalReleaseHistory(record); err != nil {
		return nil, err
	}
	if err := validateGovernedHospitalStages(record); err != nil {
		return nil, err
	}
	if err := validateGovernedHospitalApproval(record); err != nil {
		return nil, err
	}
	if record.ProtectedCount != nil {
		digest, err := record.ProtectedCount.Digest()
		if err != nil || (record.Binding.EventID != "" && digest != record.Binding.ProtectedPayloadDigest) {
			return nil, ErrGovernedHospitalJobInvalid
		}
	}
	return &GovernedHospitalJob{record: cloneGovernedHospitalJob(record)}, nil
}

func (j *GovernedHospitalJob) Record() GovernedHospitalJobRecord {
	return cloneGovernedHospitalJob(j.record)
}

func (j *GovernedHospitalJob) Resume(task GovernedHospitalTask, state *HospitalGovernance, now time.Time) error {
	fingerprint, err := task.Fingerprint()
	if err != nil || fingerprint != j.record.Fingerprint || !governedHospitalCurrentConnection(task, state) {
		return ErrGovernedHospitalJobInvalid
	}
	active := j.record.Phase == GovernedHospitalExecuting || j.record.Phase == GovernedHospitalReady || j.record.Phase == GovernedHospitalWaitingApproval
	phase, reason := "", ""
	if active && task.Validate(state, now, false) != nil {
		phase, reason = governedHospitalLocalInvalidation(task, state)
		if phase == "" {
			return ErrGovernedHospitalJobInvalid
		}
	}
	resumed := &GovernedHospitalJob{record: cloneGovernedHospitalJob(j.record)}
	if resumed.record.Task.SessionEpoch != task.SessionEpoch {
		if resumed.record.Revision == math.MaxInt64 {
			return ErrGovernedHospitalJobInvalid
		}
		resumed.record.Task.SessionEpoch = task.SessionEpoch
		resumed.record.Revision++
	}
	// Reconnect may report a durable refusal, never renew revoked authority.
	// Terminate also refuses to overwrite uncertain egress needing a receipt.
	if phase != "" {
		if err := resumed.Terminate(phase, reason, now); err != nil {
			return err
		}
	}
	j.record = resumed.record
	return nil
}

func validGovernedHospitalExecutionClock(record GovernedHospitalJobRecord) bool {
	if record.ExecutionStartedAt.IsZero() {
		return validGovernedHospitalInitialRefusal(record)
	}
	return record.ExecutionStartedAt.Before(record.Task.DispatchDeadline) && record.ExecutionDeadline.After(record.ExecutionStartedAt) && !record.ExecutionDeadline.After(record.ExecutionStartedAt.Add(30*time.Second)) && !record.ExecutionDeadline.After(record.Task.QueryDeadline) && record.ExecutionDurationMilliseconds <= 30000
}

func (t GovernedHospitalTask) Fingerprint() (string, error) {
	// Reconnect changes transport authority, not the immutable accepted job.
	t.SessionEpoch = ""
	return types.GovernanceDigest("governed-hospital-task/v1", t)
}

func (t GovernedHospitalTask) Clone() GovernedHospitalTask {
	return cloneGovernedHospitalJob(GovernedHospitalJobRecord{Task: t}).Task
}

func (t GovernedHospitalTask) Validate(state *HospitalGovernance, now time.Time, initial bool) error {
	if t.ValidateEnvelope() != nil {
		return ErrGovernedHospitalJobInvalid
	}
	if state != nil && state.QueryInvalidated(t.TenantID, t.QueryID) {
		return ErrGovernedHospitalJobInvalid
	}
	if state == nil || !state.Synchronized() || state.record.Paused || state.ValidatePolicy() != nil || !validGovernedUUID(t.JobID) || !validGovernedUUID(t.QueryID) || !validGovernedUUID(t.PrincipalID) || !validGovernedUUID(t.TenantID) || t.NodeID != state.record.NodeID || t.TenantID != t.Permit.TenantID || t.SessionEpoch != state.record.Snapshot.SessionEpoch || !hospitalHash(t.AuthorizationSnapshotHash) || t.Purpose != "research" || t.Purpose != t.Permit.Purpose || !slices.Contains(t.Permit.MemberPrincipalIDs, t.PrincipalID) || !validHospitalReleaseMode(t.ReleaseMode) || !now.Before(t.QueryDeadline) || t.QueryDeadline.After(t.Permit.ExpiresAt) {
		return ErrGovernedHospitalJobInvalid
	}
	hash, err := t.Criteria.GovernedDefinitionHash()
	if err != nil || hash != t.DefinitionHash || !validHospitalPermit(t.Permit, t.NodeID) {
		return ErrGovernedHospitalJobInvalid
	}
	a := state.CurrentAcceptance(t.TenantID, t.Permit.ID, t.Permit.Version, now)
	if a == nil || a.PermitHash != t.Permit.PermitHash || a.Revision != t.AcceptanceRevision {
		return ErrGovernedHospitalJobInvalid
	}
	policy := state.record.Policy
	// Query lifetime is shared: another target may require MANUAL approval.
	// This does not extend this Node's independent 30-second execution budget.
	maxLifetime := 48*time.Hour + 90*time.Second
	if initial && (!now.Before(t.DispatchDeadline) || t.DispatchDeadline.After(now.Add(30*time.Second)) || t.DispatchDeadline.After(t.QueryDeadline) || t.QueryDeadline.After(now.Add(maxLifetime)) || policy.Version != t.LocalPolicyVersion || policy.PolicyHash != t.LocalPolicyHash || policy.ReleaseMode != t.ReleaseMode || t.EffectiveK != max(state.record.Snapshot.NetworkFloor, policy.LocalK)) {
		return ErrGovernedHospitalJobInvalid
	}
	for _, condition := range t.Criteria.Conditions {
		if !hospitalFieldSubset([]string{condition.FieldCode}, a.AllowedFieldCodes) || !hospitalFieldSubset([]string{condition.FieldCode}, policy.FilterFieldCodes) {
			return ErrGovernedHospitalJobInvalid
		}
	}
	for _, panel := range t.Criteria.RequiredPanels {
		if !hospitalFieldSubset(panel.FieldCodes, a.AllowedFieldCodes) || !hospitalFieldSubset(panel.FieldCodes, policy.PanelFieldCodes) {
			return ErrGovernedHospitalJobInvalid
		}
	}
	return nil
}

func (j *GovernedHospitalJob) Prepare(raw uint64, state *HospitalGovernance, now time.Time) error {
	if j.record.Phase != GovernedHospitalExecuting || !now.Before(j.record.ExecutionDeadline) || j.record.Task.Validate(state, now, false) != nil || j.record.Revision == math.MaxInt64 || (j.record.Task.ReleaseMode == HospitalReleaseManual && len(j.record.Stages) >= 100) {
		return ErrGovernedHospitalJobInvalid
	}
	count, err := types.ProtectGovernedCount(raw, max(j.record.Task.EffectiveK, state.record.Snapshot.NetworkFloor, state.record.Policy.LocalK))
	if err != nil {
		return err
	}
	j.record.ProtectedCount = &count
	j.record.Phase, j.record.DeliveryState = GovernedHospitalReady, GovernedHospitalPrepared
	j.record.ExecutionDurationMilliseconds = uint64(max(int64(0), now.Sub(j.record.ExecutionStartedAt).Milliseconds()))
	if j.record.Task.ReleaseMode == HospitalReleaseManual {
		j.record.Phase = GovernedHospitalWaitingApproval
		if err := j.appendApprovalStage(now); err != nil {
			return err
		}
	}
	j.record.Revision++
	return nil
}

func (j *GovernedHospitalJob) appendApprovalStage(now time.Time) error {
	digest, err := j.record.ProtectedCount.Digest()
	if err != nil {
		return err
	}
	t := j.record.Task
	binding := GovernedHospitalBinding{TenantID: t.TenantID, NodeID: t.NodeID, QueryID: t.QueryID, JobID: t.JobID, EventID: uuid.NewString(), ProtectedPayloadDigest: digest, DefinitionHash: t.DefinitionHash, PermitID: t.Permit.ID, PermitVersion: t.Permit.Version, PermitHash: t.Permit.PermitHash, LocalPolicyVersion: t.LocalPolicyVersion, LocalPolicyHash: t.LocalPolicyHash, AcceptanceRevision: t.AcceptanceRevision, AuthorizationSnapshotHash: t.AuthorizationSnapshotHash, SessionEpoch: t.SessionEpoch}
	j.record.Stages = append(j.record.Stages, GovernedHospitalStage{Binding: binding, Sequence: int64(len(j.record.Stages) + 1), Phase: GovernedHospitalWaitingApproval, OccurredAt: now.UTC(), ExecutionDurationMilliseconds: j.record.ExecutionDurationMilliseconds})
	return nil
}

func (j *GovernedHospitalJob) ReleaseIntent(state *HospitalGovernance, now time.Time) (GovernedHospitalBinding, error) {
	if j.record.Phase != GovernedHospitalReady || j.record.ProtectedCount == nil || j.record.DeliveryState == GovernedHospitalSentUnconfirmed || j.record.Task.Validate(state, now, false) != nil {
		return GovernedHospitalBinding{}, ErrGovernedHospitalJobInvalid
	}
	if j.record.Task.ReleaseMode == HospitalReleaseManual && !j.validApprovalForCurrentPayload(state, now) {
		return GovernedHospitalBinding{}, ErrGovernedHospitalJobInvalid
	}
	if j.record.DeliveryState == GovernedHospitalPrepared && j.record.Binding.EventID != "" && j.record.Binding.LocalPolicyHash == state.record.Policy.PolicyHash && j.record.Binding.SessionEpoch == state.record.Snapshot.SessionEpoch {
		return j.record.Binding, nil
	}
	if j.record.Revision == math.MaxInt64 || len(j.record.Stages) >= 100 {
		return GovernedHospitalBinding{}, ErrGovernedHospitalJobInvalid
	}
	// Re-protection only uses previously disclosed data. Lowering k never unmasks.
	k := max(j.record.ProtectedCount.EffectiveK(), state.record.Policy.LocalK, state.record.Snapshot.NetworkFloor)
	count, err := types.NewGovernedSuppressedCount(k)
	if value, exact := j.record.ProtectedCount.ExactValue(); exact {
		count, err = types.ProtectGovernedCount(value, k)
	}
	if err != nil {
		return GovernedHospitalBinding{}, err
	}
	digest, err := count.Digest()
	if err != nil {
		return GovernedHospitalBinding{}, err
	}
	j.record.ProtectedCount = &count
	t := j.record.Task
	j.record.Binding = GovernedHospitalBinding{TenantID: t.TenantID, NodeID: t.NodeID, QueryID: t.QueryID, JobID: t.JobID, EventID: uuid.NewString(), ProtectedPayloadDigest: digest, DefinitionHash: t.DefinitionHash, PermitID: t.Permit.ID, PermitVersion: t.Permit.Version, PermitHash: t.Permit.PermitHash, LocalPolicyVersion: state.record.Policy.Version, LocalPolicyHash: state.record.Policy.PolicyHash, AcceptanceRevision: t.AcceptanceRevision, AuthorizationSnapshotHash: t.AuthorizationSnapshotHash, SessionEpoch: state.record.Snapshot.SessionEpoch}
	j.record.Grant = nil
	j.record.Receipt = nil
	j.record.Denial = nil
	j.record.DeliveryState, j.record.Reason = GovernedHospitalPrepared, ""
	j.record.Revision++
	j.appendReadyStage(now)
	return j.record.Binding, nil
}

func (j *GovernedHospitalJob) AuthorizeSend(grant GovernedHospitalGrant, state *HospitalGovernance, now time.Time) error {
	if j.record.Phase != GovernedHospitalReady || j.record.DeliveryState != GovernedHospitalPrepared || j.record.Binding.EventID == "" || grant.Binding != j.record.Binding || !validGovernedUUID(grant.ID) || grant.GrantedAt.IsZero() || grant.GrantedAt.After(now) || !now.Before(grant.ExpiresAt) || !grant.ExpiresAt.After(grant.GrantedAt) || grant.ExpiresAt.After(grant.GrantedAt.Add(10*time.Second)) || grant.ExpiresAt.After(j.record.Task.QueryDeadline) || j.record.Task.Validate(state, now, false) != nil || grant.Binding.LocalPolicyHash != state.record.Policy.PolicyHash || grant.Binding.SessionEpoch != state.record.Snapshot.SessionEpoch || j.record.Revision == math.MaxInt64 {
		return ErrGovernedHospitalJobInvalid
	}
	copyGrant := grant
	j.record.Grant, j.record.DeliveryState = &copyGrant, GovernedHospitalSentUnconfirmed
	j.record.Revision++
	return nil
}

func (j *GovernedHospitalJob) Receive(receipt GovernedHospitalReceipt) error {
	if !validGovernedUUID(receipt.ID) || receipt.JobID != j.record.Task.JobID || receipt.EventID != j.record.Binding.EventID || receipt.ProtectedPayloadDigest != j.record.Binding.ProtectedPayloadDigest || receipt.CommittedAt.IsZero() || j.record.Grant == nil || !validHospitalReceiptOutcome(receipt) || !validGovernedHospitalReceiptTime(receipt, *j.record.Grant) {
		return ErrGovernedHospitalJobInvalid
	}
	if j.record.Receipt != nil {
		prior := *j.record.Receipt
		prior.Outcome = normalizedHospitalReceiptOutcome(prior.Outcome)
		receipt.Outcome = normalizedHospitalReceiptOutcome(receipt.Outcome)
		if prior != receipt {
			return ErrGovernedHospitalJobInvalid
		}
		return nil
	}
	if j.record.DeliveryState != GovernedHospitalSentUnconfirmed || j.record.Revision == math.MaxInt64 {
		return ErrGovernedHospitalJobInvalid
	}
	copyReceipt := receipt
	j.record.Receipt = &copyReceipt
	j.record.DeliveryState, j.record.Phase = GovernedHospitalReceived, GovernedHospitalSucceeded
	if receipt.Outcome == GovernedHospitalRejected {
		j.record.DeliveryState, j.record.Phase, j.record.Reason = GovernedHospitalRejected, GovernedHospitalPolicyDenied, receipt.Reason
		if retryableGovernedHospitalReceipt(receipt.Reason) {
			j.record.Phase = GovernedHospitalReady
		}
	}
	j.record.Revision++
	return nil
}

func validHospitalReceiptOutcome(r GovernedHospitalReceipt) bool {
	if r.Outcome == GovernedHospitalCommitted || r.Outcome == "DUPLICATE_COMMITTED" {
		return r.Reason == ""
	}
	return r.Outcome == GovernedHospitalRejected && ValidHospitalGovernanceReason(r.Reason)
}

func ValidHospitalGovernanceReason(value string) bool {
	switch value {
	case types.GovernanceReasonPermitRevoked, types.GovernanceReasonPermitSuperseded, "PERMIT_EXPIRED", "ACCEPTANCE_REQUIRED", "SCOPE_DENIED", "LOCAL_POLICY_CHANGED", "HOSPITAL_PAUSED", types.GovernanceReasonQueryCanceled, types.GovernanceReasonQueryExpired, "SESSION_STALE", "GRANT_EXPIRED", "PAYLOAD_CONFLICT", "PRIVACY_REJECTED", "INTERNAL_FAILURE", "GOVERNANCE_UNSUPPORTED", "APPROVAL_DECLINED", "LOCAL_DECLINED", "APPROVAL_DEADLINE_REACHED":
		return true
	default:
		return false
	}
}

func normalizedHospitalReceiptOutcome(value string) string {
	if value == "DUPLICATE_COMMITTED" {
		return GovernedHospitalCommitted
	}
	return value
}

func governedHospitalPhase(value string) bool {
	return value == GovernedHospitalExecuting || value == GovernedHospitalReady || value == GovernedHospitalWaitingApproval || governedHospitalTerminalPhase(value)
}

func validGovernedUUID(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && parsed != uuid.Nil && parsed.String() == value
}

func hospitalHash(value string) bool {
	if len(value) != 71 || value[:7] != "sha256:" {
		return false
	}
	for _, c := range value[7:] {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func minHospitalDeadline(a, b time.Time) time.Time {
	if a.Before(b) {
		return a.UTC()
	}
	return b.UTC()
}

func cloneGovernedHospitalJob(r GovernedHospitalJobRecord) GovernedHospitalJobRecord {
	r.Stages = cloneGovernedHospitalStages(r.Stages)
	r.Task.Permit.MemberPrincipalIDs = slices.Clone(r.Task.Permit.MemberPrincipalIDs)
	r.Task.Permit.AllowedNodeIDs = slices.Clone(r.Task.Permit.AllowedNodeIDs)
	r.Task.Permit.AllowedFieldCodes = slices.Clone(r.Task.Permit.AllowedFieldCodes)
	r.Task.Criteria.Conditions = slices.Clone(r.Task.Criteria.Conditions)
	r.Task.Criteria.RequiredPanels = slices.Clone(r.Task.Criteria.RequiredPanels)
	for i := range r.Task.Criteria.RequiredPanels {
		r.Task.Criteria.RequiredPanels[i].FieldCodes = slices.Clone(r.Task.Criteria.RequiredPanels[i].FieldCodes)
	}
	if r.ProtectedCount != nil {
		count := *r.ProtectedCount
		r.ProtectedCount = &count
	}
	if r.Grant != nil {
		grant := *r.Grant
		r.Grant = &grant
	}
	if r.Receipt != nil {
		receipt := *r.Receipt
		r.Receipt = &receipt
	}
	if r.Denial != nil {
		denial := *r.Denial
		r.Denial = &denial
	}
	if r.Approval != nil {
		approval := *r.Approval
		r.Approval = &approval
	}
	return r
}
