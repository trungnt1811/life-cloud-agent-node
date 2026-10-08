package wire

import (
	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
)

func GovernedHospitalDenialFromProto(p *nodev1.ReleaseDenied) (entities.GovernedHospitalDenial, error) {
	if p == nil || governedPacketUnknown(p.ProtoReflect()) {
		return entities.GovernedHospitalDenial{}, entities.ErrGovernedHospitalJobInvalid
	}
	b, err := governedHospitalBindingFromProto(p.Binding)
	if err != nil {
		return entities.GovernedHospitalDenial{}, err
	}
	reason, valid := governedHospitalReasonFromProto(p.Reason)
	if !valid {
		return entities.GovernedHospitalDenial{}, entities.ErrGovernedHospitalJobInvalid
	}
	d := entities.GovernedHospitalDenial{Binding: b, Reason: reason}
	return d, d.Validate()
}
