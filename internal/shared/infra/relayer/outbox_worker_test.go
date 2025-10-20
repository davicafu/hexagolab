package relayer

import (
	"context"
	"errors"
	"reflect"
	"testing"

	sharedDomain "github.com/davicafu/hexagolab/internal/shared/domain"
	sharedDomainEvents "github.com/davicafu/hexagolab/internal/shared/domain/events"
	sharedBus "github.com/davicafu/hexagolab/internal/shared/infra/platform/bus"
	userDomain "github.com/davicafu/hexagolab/internal/user/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"

	"github.com/davicafu/hexagolab/tests/mocks"
)

func TestOutboxWorker_ProcessBatch_Success(t *testing.T) {
	// ARRANGE
	repo := new(mocks.MockOutboxRepository)
	publisher := new(mocks.MockPublisher)

	eventID := uuid.New()
	testEvent := sharedDomain.OutboxEvent{
		ID:        eventID,
		EventType: userDomain.UserCreated, // Usamos la constante del dominio
		Payload:   map[string]interface{}{"id": "some-uuid", "email": "test@example.com"},
	}

	// ✅ Creamos el registro con el struct EventMetadata correcto.
	registry := map[string]sharedDomainEvents.EventMetadata{
		userDomain.UserCreated: {
			Type:  reflect.TypeOf(userDomain.User{}),
			Topic: userDomain.UserTopic,
		},
	}

	// ✅ Definimos las expectativas con la nueva firma de Publish.
	repo.On("FetchPendingOutbox", mock.Anything, 10).Return([]sharedDomain.OutboxEvent{testEvent}, nil).Once()
	publisher.On("Publish", mock.Anything, mock.AnythingOfType("*domain.User")).Return(nil).Once()
	repo.On("MarkOutboxProcessed", mock.Anything, eventID).Return(nil).Once()

	worker := NewOutboxWorker(repo, publisher, registry, 0, 10, zap.NewNop())

	// ACT
	worker.ProcessBatch(context.Background())

	// ASSERT
	repo.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

func TestOutboxWorker_ProcessBatch_PublisherFails(t *testing.T) {
	// ARRANGE
	repo := new(mocks.MockOutboxRepository)
	publisher := new(mocks.MockPublisher)

	eventID := uuid.New()
	testEvent := sharedDomain.OutboxEvent{ID: eventID, EventType: userDomain.UserCreated, Payload: map[string]interface{}{}}

	registry := map[string]sharedDomainEvents.EventMetadata{
		userDomain.UserCreated: {
			Type:  reflect.TypeOf(userDomain.User{}),
			Topic: userDomain.UserTopic,
		},
	}

	repo.On("FetchPendingOutbox", mock.Anything, 10).Return([]sharedDomain.OutboxEvent{testEvent}, nil).Once()
	// ✅ Simulamos el fallo de Publish con la nueva firma.
	publisher.On("Publish", mock.Anything, mock.Anything).Return(errors.New("kafka is down")).Once()

	worker := NewOutboxWorker(repo, publisher, registry, 0, 10, zap.NewNop())

	// ACT
	worker.ProcessBatch(context.Background())

	// ASSERT
	repo.AssertCalled(t, "FetchPendingOutbox", mock.Anything, 10)
	publisher.AssertCalled(t, "Publish", mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "MarkOutboxProcessed", mock.Anything, mock.Anything)
}

func TestOutboxWorker_ProcessBatch_UnknownEventType(t *testing.T) {
	// ARRANGE
	repo := new(mocks.MockOutboxRepository)
	publisher := new(mocks.MockPublisher)

	testEvent := sharedDomain.OutboxEvent{ID: uuid.New(), EventType: "unregistered.event", Payload: map[string]interface{}{}}

	registry := make(map[string]sharedDomainEvents.EventMetadata) // Registro vacío

	repo.On("FetchPendingOutbox", mock.Anything, 10).Return([]sharedDomain.OutboxEvent{testEvent}, nil).Once()

	worker := NewOutboxWorker(repo, publisher, registry, 0, 10, zap.NewNop())

	// ACT
	worker.ProcessBatch(context.Background())

	// ASSERT
	repo.AssertExpectations(t)
	publisher.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "MarkOutboxProcessed", mock.Anything, mock.Anything)
}

// Verificación estática de que los mocks cumplen las interfaces.
var _ sharedDomain.OutboxRepository = (*mocks.MockOutboxRepository)(nil)
var _ sharedBus.EventBus = (*mocks.MockPublisher)(nil)
