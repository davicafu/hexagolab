package mocks

import (
	"context"

	sharedDomain "github.com/davicafu/hexagolab/shared/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// --- Mocks Correctos ---

// MockOutboxRepository solo implementa la interfaz que el Worker necesita.
type MockOutboxRepository struct {
	mock.Mock
}

func (m *MockOutboxRepository) FetchPendingOutbox(ctx context.Context, limit int) ([]sharedDomain.OutboxEvent, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]sharedDomain.OutboxEvent), args.Error(1)
}

func (m *MockOutboxRepository) MarkOutboxProcessed(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// MockPublisher simula el publicador de eventos con la firma correcta.
type MockPublisher struct {
	mock.Mock
}

// ✅ Firma del método Publish corregida (sin 'topic').
func (m *MockPublisher) Publish(ctx context.Context, event interface{}) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}
