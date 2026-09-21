package models

import (
	"time"

	"github.com/google/uuid"
)

// JobProgress is the GORM persistence model for the job_progress table.
type JobProgress struct {
	JobID         string     `gorm:"column:job_id;type:varchar(255);primaryKey"`
	CriteriaHash  string     `gorm:"column:criteria_hash;type:varchar(64);not null"`
	LastPatientID *uuid.UUID `gorm:"column:last_patient_id;type:uuid"`
	RunningCount  int64      `gorm:"column:running_count;type:bigint;not null;default:0"`
	Status        string     `gorm:"column:status;type:varchar(16);not null;default:in_progress"`
	UpdatedAt     time.Time  `gorm:"column:updated_at;type:timestamp with time zone;not null;default:CURRENT_TIMESTAMP"`
}

func (JobProgress) TableName() string {
	return "job_progress"
}
