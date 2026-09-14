package services

import (
	"context"
	"time"

	"github.com/lifenetwork-ai/go-backend-template/internal/domain/entities"
	"github.com/lifenetwork-ai/go-backend-template/internal/domain/usecases/services"
	"github.com/lifenetwork-ai/go-backend-template/internal/platform/logger"
)

// ExampleService is a sample external service adapter.
//
// Replace this implementation with real HTTP/RPC/client calls when a usecase
// needs an external dependency. The domain should keep depending on
// services.ExampleService only.
type ExampleService struct {
	logger logger.Logger
}

// NewExampleService creates a new ExampleService.
func NewExampleService(logger logger.Logger) services.ExampleService {
	return &ExampleService{
		logger: logger,
	}
}

// ProcessEntity simulates processing an example entity.
func (s *ExampleService) ProcessEntity(ctx context.Context, entity *entities.ExampleEntity) (*entities.ExampleEntity, error) {
	if entity == nil {
		return nil, nil
	}
	s.logger.Info("Processing example entity", logger.String("id", entity.ID().String()))

	// Simulate some asynchronous processing or external API call
	processedEntity := entities.NewExampleEntity(entity.ID(), entity.Name()+"-processed", entity.Description(), time.Now().UTC())

	return processedEntity, nil
}
