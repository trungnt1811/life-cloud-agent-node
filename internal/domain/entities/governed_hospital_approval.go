package entities

import (
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
)

const (
	HospitalApprovalApproved = "APPROVED"
	HospitalApprovalDeclined = "DECLINED"
)

type GovernedHospitalApproval struct {
	CommandID, Actor, Decision, Reason string
	ExpectedRevision, Revision         int64
	PayloadDigest, PolicyHash          string
	PolicyVersion                      int64
	DecidedAt                          time.Time
}

func (j *GovernedHospitalJob) Approve(expectedRevision int64, commandID, actor string, state *HospitalGovernance, now time.Time) error {
	return j.decideApproval(expectedRevision, commandID, actor, HospitalApprovalApproved, "", state, now)
}

func (j *GovernedHospitalJob) Decline(expectedRevision int64, commandID, actor string, state *HospitalGovernance, now time.Time) error {
	return j.decideApproval(expectedRevision, commandID, actor, HospitalApprovalDeclined, types.GovernanceReasonLocalDeclined, state, now)
}

func (j *GovernedHospitalJob) decideApproval(expectedRevision int64, commandID, actor, decision, reason string, state *HospitalGovernance, now time.Time) error {
	if prior := j.record.Approval; prior != nil && prior.CommandID == commandID {
		if prior.ExpectedRevision == expectedRevision && prior.Actor == actor && prior.Decision == decision && prior.Reason == reason {
			return nil
		}
		return ErrGovernedHospitalJobInvalid
	}
	if !validGovernedUUID(commandID) || !governedHospitalIdentifier(actor) || expectedRevision < 1 || expectedRevision != j.record.Revision || j.record.Revision == math.MaxInt64 || j.record.Phase != GovernedHospitalWaitingApproval || j.record.Task.ReleaseMode != HospitalReleaseManual || j.record.ApprovalDeadline.IsZero() || !now.Before(j.record.ApprovalDeadline) || len(j.record.Stages) >= 100 || j.record.ProtectedCount == nil || !j.waitingStageAcknowledged() || state == nil || state.record.Paused || state.record.Policy.ReleaseMode != HospitalReleaseManual || state.record.Policy.Version != j.record.Task.LocalPolicyVersion || state.record.Policy.PolicyHash != j.record.Task.LocalPolicyHash || j.record.Task.Validate(state, now, false) != nil {
		return ErrGovernedHospitalJobInvalid
	}
	digest, err := j.record.ProtectedCount.Digest()
	if err != nil {
		return ErrGovernedHospitalJobInvalid
	}
	j.record.Revision++
	j.record.Approval = &GovernedHospitalApproval{CommandID: commandID, Actor: actor, Decision: decision, Reason: reason, ExpectedRevision: expectedRevision, Revision: j.record.Revision, PayloadDigest: digest, PolicyHash: state.record.Policy.PolicyHash, PolicyVersion: state.record.Policy.Version, DecidedAt: now.UTC()}
	if decision == HospitalApprovalApproved {
		j.record.Phase = GovernedHospitalReady
		if err := j.appendApprovalReadyStage(now); err != nil {
			return err
		}
	} else {
		j.record.Phase, j.record.Reason = GovernedHospitalDeclined, reason
		j.appendTerminalStage(now)
	}
	j.record = cloneGovernedHospitalJob(j.record)
	return nil
}

func (j *GovernedHospitalJob) ExpireApproval(now time.Time) error {
	if j.record.Phase != GovernedHospitalWaitingApproval || j.record.ApprovalDeadline.IsZero() || now.Before(j.record.ApprovalDeadline) {
		return ErrGovernedHospitalJobInvalid
	}
	return j.Terminate(GovernedHospitalApprovalExpired, "APPROVAL_DEADLINE_REACHED", now)
}

func (j *GovernedHospitalJob) waitingStageAcknowledged() bool {
	for i := len(j.record.Stages) - 1; i >= 0; i-- {
		stage := j.record.Stages[i]
		if stage.Phase == GovernedHospitalWaitingApproval {
			return stage.Acknowledged && stage.AcknowledgedApprovalDeadline != nil && stage.AcknowledgedApprovalDeadline.Equal(j.record.ApprovalDeadline)
		}
	}
	return false
}

func (j *GovernedHospitalJob) appendApprovalReadyStage(now time.Time) error {
	digest, err := j.record.ProtectedCount.Digest()
	if err != nil {
		return err
	}
	t := j.record.Task
	binding := GovernedHospitalBinding{TenantID: t.TenantID, NodeID: t.NodeID, QueryID: t.QueryID, JobID: t.JobID, EventID: uuid.NewString(), ProtectedPayloadDigest: digest, DefinitionHash: t.DefinitionHash, PermitID: t.Permit.ID, PermitVersion: t.Permit.Version, PermitHash: t.Permit.PermitHash, LocalPolicyVersion: t.LocalPolicyVersion, LocalPolicyHash: t.LocalPolicyHash, AcceptanceRevision: t.AcceptanceRevision, AuthorizationSnapshotHash: t.AuthorizationSnapshotHash, SessionEpoch: t.SessionEpoch}
	j.record.Stages = append(j.record.Stages, GovernedHospitalStage{Binding: binding, Sequence: int64(len(j.record.Stages) + 1), Phase: GovernedHospitalReady, OccurredAt: now.UTC(), ExecutionDurationMilliseconds: j.record.ExecutionDurationMilliseconds})
	return nil
}

func (j *GovernedHospitalJob) validApprovalForCurrentPayload(state *HospitalGovernance, now time.Time) bool {
	approval := j.record.Approval
	if approval == nil || approval.Decision != HospitalApprovalApproved || approval.Reason != "" || j.record.Task.ReleaseMode != HospitalReleaseManual || j.record.ApprovalDeadline.IsZero() || !now.Before(j.record.ApprovalDeadline) || state == nil || state.record.Paused || state.record.Policy.ReleaseMode != HospitalReleaseManual || state.record.Policy.Version != approval.PolicyVersion || state.record.Policy.PolicyHash != approval.PolicyHash || state.record.Policy.Version != j.record.Task.LocalPolicyVersion || state.record.Policy.PolicyHash != j.record.Task.LocalPolicyHash {
		return false
	}
	digest, err := j.record.ProtectedCount.Digest()
	return err == nil && digest == approval.PayloadDigest && j.waitingStageAcknowledged()
}

func validGovernedHospitalApproval(a *GovernedHospitalApproval, task GovernedHospitalTask, count *types.GovernedProtectedCount, deadline time.Time, recordRevision int64) bool {
	if a == nil || count == nil || !validGovernedUUID(a.CommandID) || !governedHospitalIdentifier(a.Actor) || (a.Decision != HospitalApprovalApproved && a.Decision != HospitalApprovalDeclined) || a.ExpectedRevision < 1 || a.Revision != a.ExpectedRevision+1 || a.Revision > recordRevision || !hospitalHash(a.PayloadDigest) || !hospitalHash(a.PolicyHash) || a.PolicyVersion < 1 || a.DecidedAt.IsZero() || deadline.IsZero() || a.DecidedAt.After(deadline) || (a.Decision == HospitalApprovalApproved && a.Reason != "") || (a.Decision == HospitalApprovalDeclined && a.Reason != types.GovernanceReasonLocalDeclined) {
		return false
	}
	digest, err := count.Digest()
	return err == nil && digest == a.PayloadDigest && task.ReleaseMode == HospitalReleaseManual
}

func validateGovernedHospitalApproval(record GovernedHospitalJobRecord) error {
	if record.Task.ReleaseMode == HospitalReleaseAuto {
		if record.Approval != nil || !record.ApprovalDeadline.IsZero() {
			return ErrGovernedHospitalJobInvalid
		}
		return nil
	}
	if record.Task.ReleaseMode != HospitalReleaseManual || (!record.ApprovalDeadline.IsZero() && record.ApprovalDeadline.After(record.Task.QueryDeadline)) {
		return ErrGovernedHospitalJobInvalid
	}
	if record.Approval != nil && !validGovernedHospitalApproval(record.Approval, record.Task, record.ProtectedCount, record.ApprovalDeadline, record.Revision) {
		return ErrGovernedHospitalJobInvalid
	}
	switch record.Phase {
	case GovernedHospitalWaitingApproval:
		if record.Approval != nil || record.ProtectedCount == nil || record.ApprovalDeadline.IsZero() && lastWaitingStageAcknowledged(record) {
			return ErrGovernedHospitalJobInvalid
		}
	case GovernedHospitalReady, GovernedHospitalSucceeded:
		if record.Approval == nil || record.Approval.Decision != HospitalApprovalApproved || record.ApprovalDeadline.IsZero() {
			return ErrGovernedHospitalJobInvalid
		}
	case GovernedHospitalDeclined:
		declined := record.Approval != nil && record.Approval.Decision == HospitalApprovalDeclined && record.Reason == types.GovernanceReasonLocalDeclined
		paused := record.Reason == types.GovernanceReasonHospitalPaused && (record.Approval == nil || record.Approval.Decision == HospitalApprovalApproved)
		if (!declined && !paused) || record.ApprovalDeadline.IsZero() {
			return ErrGovernedHospitalJobInvalid
		}
	case GovernedHospitalApprovalExpired:
		if record.Approval != nil || record.Reason != "APPROVAL_DEADLINE_REACHED" || record.ApprovalDeadline.IsZero() {
			return ErrGovernedHospitalJobInvalid
		}
	}
	return nil
}

func lastWaitingStageAcknowledged(record GovernedHospitalJobRecord) bool {
	for i := len(record.Stages) - 1; i >= 0; i-- {
		if record.Stages[i].Phase == GovernedHospitalWaitingApproval {
			return record.Stages[i].Acknowledged
		}
	}
	return false
}
