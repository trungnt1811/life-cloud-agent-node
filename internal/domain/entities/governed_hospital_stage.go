package entities

import (
	"math"
	"slices"
	"time"

	"github.com/google/uuid"
)

type GovernedHospitalStage struct {
	Binding                       GovernedHospitalBinding
	Sequence                      int64
	Phase, Reason                 string
	OccurredAt                    time.Time
	ExecutionDurationMilliseconds uint64
	Acknowledged                  bool
	AcknowledgedExecutionDeadline *time.Time
	AcknowledgedApprovalDeadline  *time.Time
}

type GovernedHospitalStageAck struct {
	JobID, EventID, SessionEpoch                       string
	Sequence                                           int64
	ExecutionDeadline, ApprovalDeadline, QueryDeadline time.Time
}

func (j *GovernedHospitalJob) PendingStages() []GovernedHospitalStage {
	output := make([]GovernedHospitalStage, 0, len(j.record.Stages))
	for _, stage := range cloneGovernedHospitalStages(j.record.Stages) {
		if !stage.Acknowledged {
			stage.Binding.SessionEpoch = j.record.Task.SessionEpoch
			output = append(output, stage)
		}
	}
	return output
}

func (j *GovernedHospitalJob) AcknowledgeStage(ack GovernedHospitalStageAck) error {
	r := j.record
	if validGovernedHospitalInitialRefusal(r) {
		return j.acknowledgeInitialRefusal(ack)
	}
	if ack.JobID != r.Task.JobID || ack.SessionEpoch != r.Task.SessionEpoch || !ack.QueryDeadline.Equal(r.Task.QueryDeadline) || ack.ExecutionDeadline.IsZero() || ack.ExecutionDeadline.After(r.Task.QueryDeadline) || !ack.ExecutionDeadline.After(r.ExecutionStartedAt) || ack.Sequence < 1 || ack.Sequence > int64(len(r.Stages)) {
		return ErrGovernedHospitalJobInvalid
	}
	index := int(ack.Sequence - 1)
	stage := r.Stages[index]
	if ack.EventID != stage.Binding.EventID {
		return ErrGovernedHospitalJobInvalid
	}
	if stage.Acknowledged {
		if stage.AcknowledgedExecutionDeadline == nil || !stage.AcknowledgedExecutionDeadline.Equal(ack.ExecutionDeadline) || !sameOptionalTime(stage.AcknowledgedApprovalDeadline, ack.ApprovalDeadline) {
			return ErrGovernedHospitalJobInvalid
		}
		return nil
	}
	if stage.Phase == GovernedHospitalWaitingApproval {
		// The approval window starts at CP persistence, after the local event.
		// Its ACK remains bounded by the already validated absolute query lifetime.
		if ack.ApprovalDeadline.IsZero() || !ack.ApprovalDeadline.After(stage.OccurredAt) || ack.ApprovalDeadline.After(r.Task.QueryDeadline) {
			return ErrGovernedHospitalJobInvalid
		}
	} else if !ack.ApprovalDeadline.IsZero() {
		return ErrGovernedHospitalJobInvalid
	}
	if r.Revision == math.MaxInt64 || (index > 0 && !r.Stages[index-1].Acknowledged) {
		return ErrGovernedHospitalJobInvalid
	}
	if index > 0 && (r.Stages[0].AcknowledgedExecutionDeadline == nil || !r.Stages[0].AcknowledgedExecutionDeadline.Equal(ack.ExecutionDeadline)) {
		return ErrGovernedHospitalJobInvalid
	}
	// The CP clock may have started after our local transaction; only the
	// earlier deadline can govern local computation. No retry restarts either.
	if ack.ExecutionDeadline.Before(r.ExecutionDeadline) {
		r.ExecutionDeadline = ack.ExecutionDeadline.UTC()
	}
	at := ack.ExecutionDeadline.UTC()
	r.Stages[index].Acknowledged, r.Stages[index].AcknowledgedExecutionDeadline = true, &at
	if stage.Phase == GovernedHospitalWaitingApproval {
		approvalDeadline := ack.ApprovalDeadline.UTC()
		r.Stages[index].AcknowledgedApprovalDeadline = &approvalDeadline
		r.ApprovalDeadline = approvalDeadline
	}
	r.Revision++
	j.record = cloneGovernedHospitalJob(r)
	return nil
}

func (j *GovernedHospitalJob) acknowledgeInitialRefusal(ack GovernedHospitalStageAck) error {
	r := j.record
	if ack.JobID != r.Task.JobID || ack.SessionEpoch != r.Task.SessionEpoch || !ack.QueryDeadline.Equal(r.Task.QueryDeadline) || !ack.ExecutionDeadline.IsZero() || ack.Sequence != 1 || ack.EventID != r.Stages[0].Binding.EventID {
		return ErrGovernedHospitalJobInvalid
	}
	if r.Stages[0].Acknowledged {
		return nil
	}
	if r.Revision == math.MaxInt64 {
		return ErrGovernedHospitalJobInvalid
	}
	r.Stages[0].Acknowledged = true
	r.Revision++
	j.record = cloneGovernedHospitalJob(r)
	return nil
}

func initialGovernedHospitalStage(task GovernedHospitalTask, now time.Time) GovernedHospitalStage {
	return GovernedHospitalStage{Binding: GovernedHospitalBinding{TenantID: task.TenantID, NodeID: task.NodeID, QueryID: task.QueryID, JobID: task.JobID, EventID: uuid.NewString(), DefinitionHash: task.DefinitionHash, PermitID: task.Permit.ID, PermitVersion: task.Permit.Version, PermitHash: task.Permit.PermitHash, LocalPolicyVersion: task.LocalPolicyVersion, LocalPolicyHash: task.LocalPolicyHash, AcceptanceRevision: task.AcceptanceRevision, AuthorizationSnapshotHash: task.AuthorizationSnapshotHash, SessionEpoch: task.SessionEpoch}, Sequence: 1, Phase: GovernedHospitalExecuting, OccurredAt: now.UTC()}
}

func (j *GovernedHospitalJob) appendReadyStage(now time.Time) {
	j.record.Stages = append(j.record.Stages, GovernedHospitalStage{Binding: j.record.Binding, Sequence: int64(len(j.record.Stages) + 1), Phase: GovernedHospitalReady, OccurredAt: now.UTC(), ExecutionDurationMilliseconds: j.record.ExecutionDurationMilliseconds})
}

func validateGovernedHospitalStages(r GovernedHospitalJobRecord) error {
	if len(r.Stages) < 1 || len(r.Stages) > 100 {
		return ErrGovernedHospitalJobInvalid
	}
	seen := make(map[string]bool, len(r.Stages))
	var pending bool
	var approvalDeadline time.Time
	for i, stage := range r.Stages {
		b, task := stage.Binding, r.Task
		if stage.Sequence != int64(i+1) || !validGovernedUUID(b.EventID) || seen[b.EventID] || b.JobID != task.JobID || b.QueryID != task.QueryID || b.NodeID != task.NodeID || b.TenantID != task.TenantID || b.DefinitionHash != task.DefinitionHash || b.PermitID != task.Permit.ID || b.PermitVersion != task.Permit.Version || b.PermitHash != task.Permit.PermitHash || b.AuthorizationSnapshotHash != task.AuthorizationSnapshotHash || b.LocalPolicyVersion < 1 || !hospitalHash(b.LocalPolicyHash) || b.AcceptanceRevision != task.AcceptanceRevision || b.SessionEpoch == "" || stage.OccurredAt.IsZero() || stage.OccurredAt.Before(r.ExecutionStartedAt) || stage.ExecutionDurationMilliseconds > 30000 {
			return ErrGovernedHospitalJobInvalid
		}
		if !validGovernedStagePhase(r, stage, i) {
			return ErrGovernedHospitalJobInvalid
		}
		if stage.Acknowledged && !validGovernedHospitalInitialRefusal(r) {
			if pending || !validGovernedStageAcknowledgement(r, stage, i) {
				return ErrGovernedHospitalJobInvalid
			}
			if stage.Phase == GovernedHospitalWaitingApproval {
				approvalDeadline = *stage.AcknowledgedApprovalDeadline
			}
		} else if !stage.Acknowledged {
			if stage.AcknowledgedExecutionDeadline != nil || stage.AcknowledgedApprovalDeadline != nil {
				return ErrGovernedHospitalJobInvalid
			}
			pending = true
		}
		seen[b.EventID] = true
	}
	if !sameOptionalTime(nonZeroTimePointer(approvalDeadline), r.ApprovalDeadline) {
		return ErrGovernedHospitalJobInvalid
	}
	return nil
}

func validGovernedStageAcknowledgement(r GovernedHospitalJobRecord, stage GovernedHospitalStage, index int) bool {
	deadline := stage.AcknowledgedExecutionDeadline
	if deadline == nil || deadline.IsZero() || !deadline.After(r.ExecutionStartedAt) || deadline.After(r.Task.QueryDeadline) {
		return false
	}
	if index > 0 && (r.Stages[0].AcknowledgedExecutionDeadline == nil || !r.Stages[0].AcknowledgedExecutionDeadline.Equal(*deadline)) {
		return false
	}
	approval := stage.AcknowledgedApprovalDeadline
	if stage.Phase == GovernedHospitalWaitingApproval {
		return approval != nil && !approval.IsZero() && approval.After(stage.OccurredAt) && !approval.After(r.Task.QueryDeadline)
	}
	return approval == nil
}

func nonZeroTimePointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

func validGovernedStagePhase(r GovernedHospitalJobRecord, stage GovernedHospitalStage, index int) bool {
	if index == 0 {
		if validGovernedHospitalInitialRefusal(r) {
			return !stage.OccurredAt.After(r.Task.QueryDeadline)
		}
		return stage.Phase == GovernedHospitalExecuting && stage.Reason == "" && stage.Binding.ProtectedPayloadDigest == "" && stage.OccurredAt.Equal(r.ExecutionStartedAt)
	}
	if !governedHospitalTerminalPhase(stage.Phase) {
		return (stage.Phase == GovernedHospitalReady || stage.Phase == GovernedHospitalWaitingApproval) && stage.Reason == "" && hospitalHash(stage.Binding.ProtectedPayloadDigest)
	}
	return stage.Phase != GovernedHospitalSucceeded && index == len(r.Stages)-1 && stage.Phase == r.Phase && stage.Reason == r.Reason && stage.Reason != "" && ValidHospitalGovernanceReason(stage.Reason) && (stage.Binding.ProtectedPayloadDigest == "" || hospitalHash(stage.Binding.ProtectedPayloadDigest))
}

func cloneGovernedHospitalStages(stages []GovernedHospitalStage) []GovernedHospitalStage {
	output := slices.Clone(stages)
	for i := range output {
		if at := output[i].AcknowledgedExecutionDeadline; at != nil {
			copyAt := *at
			output[i].AcknowledgedExecutionDeadline = &copyAt
		}
		if at := output[i].AcknowledgedApprovalDeadline; at != nil {
			copyAt := *at
			output[i].AcknowledgedApprovalDeadline = &copyAt
		}
	}
	return output
}

func sameOptionalTime(value *time.Time, other time.Time) bool {
	if value == nil {
		return other.IsZero()
	}
	return !other.IsZero() && value.Equal(other)
}
