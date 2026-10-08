package models

import "time"

type HospitalGovernanceState struct {
	NodeID string `gorm:"column:node_id;primaryKey"`
	State  string `gorm:"column:state;type:jsonb;not null"`
}

func (HospitalGovernanceState) TableName() string { return "hospital_governance_states" }

type HospitalGovernanceCommand struct {
	NodeID      string    `gorm:"column:node_id;primaryKey"`
	Actor       string    `gorm:"column:actor;primaryKey"`
	CommandKey  string    `gorm:"column:command_key;primaryKey"`
	Fingerprint string    `gorm:"column:fingerprint"`
	Response    string    `gorm:"column:response;type:jsonb"`
	CreatedAt   time.Time `gorm:"column:created_at"`
}

func (HospitalGovernanceCommand) TableName() string { return "hospital_governance_commands" }

type HospitalGovernanceAudit struct {
	EventID          string    `gorm:"column:event_id;primaryKey"`
	NodeID           string    `gorm:"column:node_id"`
	Type             string    `gorm:"column:event_type"`
	Actor            string    `gorm:"column:actor"`
	LocalRevision    int64     `gorm:"column:local_revision"`
	SnapshotRevision int64     `gorm:"column:snapshot_revision"`
	PermitID         string    `gorm:"column:permit_id"`
	PermitVersion    int64     `gorm:"column:permit_version"`
	PolicyHash       string    `gorm:"column:policy_hash"`
	OccurredAt       time.Time `gorm:"column:occurred_at"`
	JobID            *string   `gorm:"column:job_id"`
	OutboundEventID  *string   `gorm:"column:outbound_event_id"`
	QueryID          *string   `gorm:"column:query_id"`
	CommandID        *string   `gorm:"column:command_id"`
}

func (HospitalGovernanceAudit) TableName() string { return "hospital_governance_audit" }
