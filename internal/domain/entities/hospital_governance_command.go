package entities

import "time"

type HospitalGovernanceCommandRecord struct {
	NodeID      string
	Actor       string
	Key         string
	Fingerprint string
	Response    HospitalGovernanceRecord
	CreatedAt   time.Time
}

type HospitalGovernanceAuditRecord struct {
	EventID          string
	NodeID           string
	Type             string
	Actor            string
	LocalRevision    int64
	SnapshotRevision int64
	PermitID         string
	PermitVersion    int64
	PolicyHash       string
	OccurredAt       time.Time
	JobID            *string
	OutboundEventID  *string
	QueryID          *string
	CommandID        *string
}
