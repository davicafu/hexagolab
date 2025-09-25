package mocks

import (
	"context"

	"github.com/davicafu/hexagolab/internal/user/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// MockRepository simula el repo con outbox
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) FetchPendingOutbox(ctx context.Context, limit int) ([]domain.OutboxEvent, error) {
	args := m.Called(ctx, limit)
	return args.Get(0).([]domain.OutboxEvent), args.Error(1)
}

func (m *MockRepository) MarkOutboxProcessed(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// MockPublisher simula un publisher
type MockPublisher struct {
	mock.Mock
}

func (m *MockPublisher) Publish(ctx context.Context, eventType string, event interface{}) error {
	args := m.Called(ctx, eventType, event)
	return args.Error(0)
}
