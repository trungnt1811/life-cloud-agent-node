package models

import "time"

type GovernedHospitalJob struct {
	NodeID           string     `gorm:"column:node_id;primaryKey"`
	JobID            string     `gorm:"column:job_id;primaryKey"`
	QueryID          string     `gorm:"column:query_id"`
	Phase            string     `gorm:"column:phase"`
	Record           string     `gorm:"column:record;type:jsonb"`
	ApprovalDeadline *time.Time `gorm:"column:approval_deadline"`
}

func (GovernedHospitalJob) TableName() string { return "governed_hospital_jobs" }

type GovernedHospitalOutboundEvent struct {
	NodeID        string    `gorm:"column:node_id;primaryKey"`
	EventID       string    `gorm:"column:event_id;primaryKey"`
	JobID         string    `gorm:"column:job_id"`
	DeliveryState string    `gorm:"column:delivery_state"`
	Record        string    `gorm:"column:record;type:jsonb"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

func (GovernedHospitalOutboundEvent) TableName() string { return "governed_hospital_outbound_events" }
