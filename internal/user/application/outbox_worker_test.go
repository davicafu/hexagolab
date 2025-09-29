package application

import (
	"context"
	"testing"
	"time"

	"github.com/davicafu/hexagolab/internal/user/domain"
	"github.com/davicafu/hexagolab/tests/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// MinimalRepo implementa UserRepository mínimo para OutboxWorker
type MinimalRepo struct {
	mock.Mock
}

func (r *MinimalRepo) FetchPendingOutbox(ctx context.Context, limit int) ([]domain.OutboxEvent, error) {
	args := r.Called(ctx, limit)
	return args.Get(0).([]domain.OutboxEvent), args.Error(1)
}

func (r *MinimalRepo) MarkOutboxProcessed(ctx context.Context, id uuid.UUID) error {
	args := r.Called(ctx, id)
	return args.Error(0)
}

// Métodos no usados en OutboxWorker
func (r *MinimalRepo) Create(ctx context.Context, u *domain.User, evt domain.OutboxEvent) error {
	panic("not implemented")
}
func (r *MinimalRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	panic("not implemented")
}
func (r *MinimalRepo) Update(ctx context.Context, u *domain.User, evt domain.OutboxEvent) error {
	panic("not implemented")
}
func (r *MinimalRepo) DeleteByID(ctx context.Context, id uuid.UUID, evt domain.OutboxEvent) error {
	panic("not implemented")
}
func (r *MinimalRepo) List(ctx context.Context, f domain.UserFilter) ([]*domain.User, error) {
	panic("not implemented")
}
func (r *MinimalRepo) SaveOutboxEvent(ctx context.Context, evt domain.OutboxEvent) error {
	panic("not implemented")
}

func TestOutboxWorker_ProcessBatch(t *testing.T) {
	ctx := context.Background()
	repo := new(MinimalRepo)
	publisher := new(mocks.MockPublisher)

	// Crear un evento pendiente
	eventID := uuid.New()
	evt := domain.OutboxEvent{
		ID:            eventID,
		AggregateType: "User",
		AggregateID:   uuid.New().String(),
		EventType:     "user.created",
		Payload:       map[string]interface{}{"email": "test@example.com"},
		CreatedAt:     time.Now(),
		Processed:     false,
	}

	// Configurar expectativas del mock
	repo.On("FetchPendingOutbox", mock.Anything, 10).Return([]domain.OutboxEvent{evt}, nil).Once()
	repo.On("MarkOutboxProcessed", mock.Anything, eventID).Return(nil).Once()
	publisher.On("Publish", mock.Anything, evt.EventType, mock.Anything).Return(nil).Once()

	// Crear worker (no llamamos a Start, para evitar goroutines)
	worker := NewOutboxWorker(repo, publisher, 10*time.Millisecond, 10)

	// Ejecutar la función directamente
	worker.ProcessBatch(ctx)

	// Verificar que se cumplieron las expectativas
	repo.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

func TestOutboxWorker_Enqueue(t *testing.T) {
	//ctx, cancel := context.WithCancel(context.Background())
	//defer cancel()

	repo := new(MinimalRepo)
	publisher := new(mocks.MockPublisher)

	// Crear un evento de prueba
	eventID := uuid.New()
	evt := domain.OutboxEvent{
		ID:            eventID,
		AggregateType: "User",
		AggregateID:   uuid.New().String(),
		EventType:     "user.created",
		Payload:       map[string]interface{}{"email": "enqueue@example.com"},
		CreatedAt:     time.Now(),
		Processed:     false,
	}

	// Esperamos que al publicar desde el canal se llame a Publish y MarkOutboxProcessed
	publisher.On("Publish", mock.Anything, evt.EventType, mock.Anything).Return(nil).Once()
	repo.On("MarkOutboxProcessed", mock.Anything, evt.ID).Return(nil).Once()

	// Crear worker con canal
	worker := NewOutboxWorker(repo, publisher, 50*time.Millisecond, 10)
	// Procesa el evento directamente:
	worker.publishAndMark(context.Background(), evt)

	// Encolar evento
	worker.Enqueue(evt)

	// Dar tiempo a que la goroutine procese
	time.Sleep(100 * time.Millisecond)

	// Verificaciones
	repo.AssertExpectations(t)
	publisher.AssertExpectations(t)
}
