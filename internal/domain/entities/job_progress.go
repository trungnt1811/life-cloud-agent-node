package entities

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// JobProgressStatus is the lifecycle state of a checkpointed job.
type JobProgressStatus string

const (
	JobProgressInProgress JobProgressStatus = "in_progress"
	JobProgressDone       JobProgressStatus = "done"
)

// JobProgress is the D4 checkpoint for one federated query job (decision 0005).
type JobProgress struct {
	jobID         string
	criteriaHash  string
	lastPatientID uuid.UUID
	runningCount  uint64
	status        JobProgressStatus
	updatedAt     time.Time
}

// JobProgressRecord is a persistence snapshot used at repository boundaries.
// LastPatientID is uuid.Nil until the first chunk completes.
type JobProgressRecord struct {
	JobID         string
	CriteriaHash  string
	LastPatientID uuid.UUID
	RunningCount  uint64
	Status        JobProgressStatus
	UpdatedAt     time.Time
}

// NewJobProgress starts a checkpoint for a job that has completed no chunk.
func NewJobProgress(jobID, criteriaHash string, now time.Time) *JobProgress {
	return &JobProgress{
		jobID:        strings.TrimSpace(jobID),
		criteriaHash: criteriaHash,
		status:       JobProgressInProgress,
		updatedAt:    ensureTimestamp(now),
	}
}

// NewJobProgressFromRecord hydrates a checkpoint from persistence data.
func NewJobProgressFromRecord(record JobProgressRecord) *JobProgress {
	if strings.TrimSpace(record.JobID) == "" {
		return nil
	}
	return &JobProgress{
		jobID:         record.JobID,
		criteriaHash:  record.CriteriaHash,
		lastPatientID: record.LastPatientID,
		runningCount:  record.RunningCount,
		status:        record.Status,
		updatedAt:     record.UpdatedAt,
	}
}

// Record returns a persistence snapshot.
func (p *JobProgress) Record() JobProgressRecord {
	if p == nil {
		return JobProgressRecord{}
	}
	return JobProgressRecord{
		JobID:         p.jobID,
		CriteriaHash:  p.criteriaHash,
		LastPatientID: p.lastPatientID,
		RunningCount:  p.runningCount,
		Status:        p.status,
		UpdatedAt:     p.updatedAt,
	}
}

// CompleteChunk advances the cursor and adds the chunk's matches. Cursor and
// count move together so they cannot disagree.
func (p *JobProgress) CompleteChunk(lastPatientID uuid.UUID, matched uint64, now time.Time) {
	if p == nil {
		return
	}
	p.lastPatientID = lastPatientID
	p.runningCount += matched
	p.updatedAt = ensureTimestamp(now)
}

// MarkDone records that every chunk has been counted.
func (p *JobProgress) MarkDone(now time.Time) {
	if p == nil {
		return
	}
	p.status = JobProgressDone
	p.updatedAt = ensureTimestamp(now)
}

func (p *JobProgress) JobID() string {
	if p == nil {
		return ""
	}
	return p.jobID
}

func (p *JobProgress) CriteriaHash() string {
	if p == nil {
		return ""
	}
	return p.criteriaHash
}

// LastPatientID is uuid.Nil until the first chunk completes.
func (p *JobProgress) LastPatientID() uuid.UUID {
	if p == nil {
		return uuid.Nil
	}
	return p.lastPatientID
}

func (p *JobProgress) RunningCount() uint64 {
	if p == nil {
		return 0
	}
	return p.runningCount
}

func (p *JobProgress) Status() JobProgressStatus {
	if p == nil {
		return ""
	}
	return p.status
}

func (p *JobProgress) Done() bool {
	return p.Status() == JobProgressDone
}
