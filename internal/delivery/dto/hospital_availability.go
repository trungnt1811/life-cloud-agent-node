package dto

import "github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"

type HospitalAvailabilityRequest struct {
	ExpectedRevision *int64 `json:"expected_revision" binding:"required"`
	Paused           *bool  `json:"paused" binding:"required"`
}

type HospitalAvailabilityDTO struct {
	HospitalSyncDTO
	Version int64 `json:"version"`
	Paused  bool  `json:"paused"`
}

func NewHospitalAvailabilityDTO(r entities.HospitalGovernanceRecord) HospitalAvailabilityDTO {
	return HospitalAvailabilityDTO{HospitalSyncDTO: hospitalSyncDTO(r), Version: r.AvailabilityRevision, Paused: r.Paused}
}
