package events

import (
	"context"
	"encoding/json"
	"errors" // Necesario para la comprobación de errores
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	sharedEvents "github.com/davicafu/hexagolab/internal/shared/domain/events"
	sharedUtils "github.com/davicafu/hexagolab/internal/shared/infra/utils"
	userDomain "github.com/davicafu/hexagolab/internal/user/domain"
)

type UserEvent struct {
	Key     string
	Payload []byte
}

type UserService interface {
	CreateUser(ctx context.Context, email, nombre string, birthDate time.Time) (*userDomain.User, error)
	UpdateUser(ctx context.Context, u *userDomain.User) error
	GetUser(ctx context.Context, id uuid.UUID) (*userDomain.User, error)
}

// UserConsumer (sin el campo batchSize)
type UserConsumer struct {
	service UserService
	log     *zap.Logger
}

// NewUserConsumer (sin el parámetro batchSize)
func NewUserConsumer(service UserService, logger *zap.Logger) *UserConsumer {
	return &UserConsumer{
		service: service,
		log:     logger,
	}
}

func (c *UserConsumer) HandleMessage(ctx context.Context, key string, payload []byte) {
	var base sharedEvents.IntegrationEvent
	if err := json.Unmarshal(payload, &base); err != nil {
		c.log.Warn("Failed to unmarshal integration event", zap.String("key", key), zap.Error(err))
		return
	}

	// ✅ Usamos las constantes en lugar de strings
	switch base.Type {
	case userDomain.UserCreated:
		sharedUtils.UnmarshalAndHandle[sharedEvents.UserCreated](c.log, base.Data, func(evt sharedEvents.UserCreated) {
			c.withContext(ctx, evt.ID, func(ctxUser context.Context) error {

				// ✅ LÓGICA DE IDEMPOTENCIA: "Buscar antes de Crear"
				// 1. Comprobamos si el usuario ya existe.
				_, err := c.service.GetUser(ctxUser, evt.ID)
				if err == nil {
					// El usuario ya existe, no hacemos nada. Es un evento duplicado.
					c.log.Info("Evento 'UserCreated' duplicado ignorado", zap.String("user_id", evt.ID.String()))
					return nil
				}
				// Si el error no es "no encontrado", es un error real que debemos devolver.
				if !errors.Is(err, userDomain.ErrUserNotFound) {
					return err
				}

				// 2. Si no existe, lo creamos.
				_, err = c.service.CreateUser(ctxUser, evt.Email, evt.Nombre, evt.BirthDate)
				return err

			}, "User created via event", evt)
		})

	case userDomain.UserUpdated:
		sharedUtils.UnmarshalAndHandle[sharedEvents.UserUpdated](c.log, base.Data, func(evt sharedEvents.UserUpdated) {
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

// Helper para ejecutar acción con contexto limitado y log
func (c *UserConsumer) withContext(ctx context.Context, id uuid.UUID, action func(ctx context.Context) error, successMsg string, evt interface{}) {
	ctxUser, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	if err := action(ctxUser); err != nil {
		// ✅ Si el error es que ya existe, lo tratamos como un éxito (alternativa de idempotencia)
		if errors.Is(err, userDomain.ErrUserAlreadyExists) {
			c.log.Info("Evento 'UserCreated' duplicado gestionado por la BBDD", zap.String("user_id", id.String()))
			return
		}

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

func BackgroundConsumerChan(ctx context.Context, ch <-chan interface{}, consumer *UserConsumer) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				consumer.log.Info("UserConsumer stopped")
				return
			case msg := <-ch:
				// ✅ Esperamos recibir []byte, que es lo que el bus envía.
				if payload, ok := msg.([]byte); ok {
					// Le pasamos los bytes directamente al handler.
					consumer.HandleMessage(ctx, "", payload)
				}
			}
		}
	}()
}
