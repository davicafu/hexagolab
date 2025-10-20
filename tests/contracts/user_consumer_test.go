package contracts

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/davicafu/hexagolab/internal/shared/domain/events"
	userDomain "github.com/davicafu/hexagolab/internal/user/domain"
	userConsumer "github.com/davicafu/hexagolab/internal/user/infra/inbound/events"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// --- FakeUserService para pruebas ---
type FakeUserService struct {
	Created []*userDomain.User
	Updated []*userDomain.User
	Users   map[uuid.UUID]*userDomain.User
}

func NewFakeUserService() *FakeUserService {
	return &FakeUserService{
		Created: []*userDomain.User{},
		Updated: []*userDomain.User{},
		Users:   make(map[uuid.UUID]*userDomain.User),
	}
}

func (f *FakeUserService) CreateUser(ctx context.Context, email, nombre string, birthDate time.Time) (*userDomain.User, error) {
	u := &userDomain.User{
		ID:        uuid.New(),
		Email:     email,
		Nombre:    nombre,
		BirthDate: birthDate,
	}
	f.Created = append(f.Created, u)
	f.Users[u.ID] = u
	return u, nil
}

func (f *FakeUserService) GetUser(ctx context.Context, id uuid.UUID) (*userDomain.User, error) {
	u, ok := f.Users[id]
	if !ok {
		return nil, userDomain.ErrUserNotFound
	}
	return u, nil
}

func (f *FakeUserService) UpdateUser(ctx context.Context, u *userDomain.User) error {
	f.Updated = append(f.Updated, u)
	f.Users[u.ID] = u
	return nil
}

// --- Test del UserConsumer ---
func TestUserConsumer_HandleMessage(t *testing.T) {
	ctx := context.Background()
	fakeService := NewFakeUserService()
	consumer := userConsumer.NewUserConsumer(fakeService, zap.NewNop())

	// Helper para crear IntegrationEvent con Data
	buildEvent := func(eventType string, data interface{}) []byte {
		raw, _ := json.Marshal(data)
		integration := events.IntegrationEvent{
			Type:      eventType,
			Timestamp: time.Now(),
			Data:      raw,
		}
		payload, _ := json.Marshal(integration)
		return payload
	}

	// --- 1. Evento UserCreated válido ---
	createdEvent := events.UserCreated{
		ID:        uuid.New(),
		Email:     "ana@example.com",
		Nombre:    "Ana",
		BirthDate: time.Now().Add(-20 * 365 * 24 * time.Hour),
	}
	payload := buildEvent("user.created", createdEvent)
	consumer.HandleMessage(ctx, "user.created", payload)

	assert.Len(t, fakeService.Created, 1)
	assert.Equal(t, "Ana", fakeService.Created[0].Nombre)
	assert.Equal(t, "ana@example.com", fakeService.Created[0].Email)

	// --- 2. Evento UserUpdated válido ---
	updatedEvent := events.UserUpdated{
		ID:        fakeService.Created[0].ID,
		Email:     "ana2@example.com",
		Nombre:    "Ana Updated",
		BirthDate: fakeService.Created[0].BirthDate,
	}
	payload = buildEvent("user.updated", updatedEvent)
	consumer.HandleMessage(ctx, "user.updated", payload)

	assert.Len(t, fakeService.Updated, 1)
	assert.Equal(t, "Ana Updated", fakeService.Updated[0].Nombre)
	assert.Equal(t, "ana2@example.com", fakeService.Updated[0].Email)

	// --- 3. Evento con payload malformado ---
	badPayload := []byte(`{"Type": "user.created", "Data": "bad json"`)
	consumer.HandleMessage(ctx, "user.created", badPayload)

	// Nada nuevo debe haberse creado
	assert.Len(t, fakeService.Created, 1)

	// --- 4. Evento con tipo desconocido ---
	unknownEvent := struct {
		Type string `json:"type"`
	}{Type: "UnknownType"}
	payload, _ = json.Marshal(unknownEvent)
	consumer.HandleMessage(ctx, "unknown.event", payload)

	// Nada nuevo debe haberse creado o actualizado
	assert.Len(t, fakeService.Created, 1)
	assert.Len(t, fakeService.Updated, 1)
}
