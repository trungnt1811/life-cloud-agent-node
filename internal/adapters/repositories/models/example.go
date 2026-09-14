package models

import (
	"time"

	"github.com/google/uuid"
)

// Example is the GORM persistence model for the examples table.
type Example struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Name        string    `gorm:"type:varchar(255);not null"`
	Description string    `gorm:"type:text"`
	CreatedAt   time.Time `gorm:"type:timestamp with time zone;default:CURRENT_TIMESTAMP"`
	UpdatedAt   time.Time `gorm:"type:timestamp with time zone;default:CURRENT_TIMESTAMP"`
}

func (Example) TableName() string {
	return "examples"
}
