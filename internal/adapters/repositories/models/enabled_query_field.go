package models

import "time"

// EnabledQueryField is the GORM persistence model for enabled_query_fields.
type EnabledQueryField struct {
	FieldCode string    `gorm:"column:field_code;type:varchar(64);primaryKey"`
	Enabled   bool      `gorm:"column:enabled;type:boolean;not null;default:false"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:timestamp with time zone;not null;default:CURRENT_TIMESTAMP"`
	UpdatedBy string    `gorm:"column:updated_by;type:varchar(255);not null;default:''"`
}

func (EnabledQueryField) TableName() string {
	return "enabled_query_fields"
}
