package entities

import "math"

func (g *HospitalGovernance) ChangeAvailability(paused bool, expected int64) error {
	if expected < 1 || expected != g.record.AvailabilityRevision {
		return ErrHospitalGovernanceConflict
	}
	if expected == math.MaxInt64 || g.record.LocalRevision == math.MaxInt64 {
		return ErrHospitalGovernanceInvalid
	}
	g.record.Paused = paused
	g.record.AvailabilityRevision++
	g.record.LocalRevision++
	return nil
}
