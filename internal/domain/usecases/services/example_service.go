package services

import (
	"context"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/entities"
)

// ExampleService is the domain port for an external example service.
//
// Implementations live under internal/adapters/services. Usecases depend on
// this contract, not on the concrete HTTP/RPC/client implementation.
type ExampleService interface {
	ProcessEntity(ctx context.Context, entity *entities.ExampleEntity) (*entities.ExampleEntity, error)
}
