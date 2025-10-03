package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/davicafu/hexagolab/internal/shared/events"
	"github.com/davicafu/hexagolab/internal/user/domain"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type UserEvent struct {
	Key     string
	Payload []byte
}

type UserService interface {
	CreateUser(ctx context.Context, email, nombre string, birthDate time.Time) (*domain.User, error)
	UpdateUser(ctx context.Context, u *domain.User) error
	GetUser(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

type UserConsumer struct {
	service   UserService
	batchSize int
	log       *zap.Logger
}

func NewUserConsumer(service UserService, batch int, logger *zap.Logger) *UserConsumer {
	return &UserConsumer{
		service:   service,
		batchSize: batch,
		log:       logger,
	}
}

func (c *UserConsumer) HandleMessage(ctx context.Context, key string, payload []byte) {

	var base events.IntegrationEvent
	if err := json.Unmarshal(payload, &base); err != nil {
		c.log.Warn("Failed to unmarshal integration event",
			zap.String("key", key),
			zap.Error(err),
		)
		return
	}

	switch base.Type {
	case "UserCreated":
		unmarshalAndHandle[events.UserCreated](c.log, base.Data, func(evt events.UserCreated) {
			c.withContext(ctx, evt.ID, func(ctxUser context.Context) error {
				_, err := c.service.CreateUser(ctxUser, evt.Email, evt.Nombre, evt.BirthDate)
				return err
			}, "User created via event", evt)
		})
	case "UserUpdated":
		unmarshalAndHandle[events.UserUpdated](c.log, base.Data, func(evt events.UserUpdated) {
			c.withContext(ctx, evt.ID, func(ctxUser context.Context) error {
				user, err := c.service.GetUser(ctxUser, evt.ID)
				if err != nil {
					return err
				}
				user.Email = evt.Email
				user.Nombre = evt.Nombre
				user.BirthDate = evt.BirthDate
				return c.service.UpdateUser(ctxUser, user)
			}, "User updated via event", evt)
		})
	default:
		c.log.Warn("Unknown event type", zap.String("type", base.Type))
	}
}

// Helper genérico para deserializar JSON y ejecutar un handler
func unmarshalAndHandle[T any](log *zap.Logger, data json.RawMessage, handler func(T)) {

	var evt T
	if err := json.Unmarshal(data, &evt); err != nil {
		log.Warn("Failed to unmarshal event data", zap.Error(err))
		return
	}
	handler(evt)
}

// Helper para ejecutar acción con contexto limitado y log
func (c *UserConsumer) withContext(ctx context.Context, id uuid.UUID, action func(ctx context.Context) error, successMsg string, evt interface{}) {

	ctxUser, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	if err := action(ctxUser); err != nil {
		c.log.Warn("Failed to process user event",
			zap.String("user_id", id.String()),
			zap.Any("event", evt),
			zap.Error(err),
		)
	} else {
		c.log.Info(successMsg,
			zap.String("user_id", id.String()),
			zap.Any("event", evt),
		)
	}
}

func BackgroundConsumerChan(ctx context.Context, ch <-chan UserEvent, consumer *UserConsumer) {

	go func() {
		for {
			select {
			case <-ctx.Done():
				consumer.log.Info("UserConsumer stopped")
				return
			case evt := <-ch:
				consumer.HandleMessage(ctx, evt.Key, evt.Payload)
			}
		}
	}()
}
