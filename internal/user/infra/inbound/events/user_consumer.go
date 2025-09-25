package events

import (
	"context"
	"log"
	"time"

	"encoding/json"

	"github.com/davicafu/hexagolab/internal/user/application"
	"github.com/davicafu/hexagolab/internal/user/domain"
)

// UserEvent representa un mensaje de usuario que llega al consumer
type UserEvent struct {
	Key     string
	Payload []byte
}

// UserConsumer procesa eventos de usuario recibidos.
type UserConsumer struct {
	service   *application.UserService
	batchSize int
}

// NewUserConsumer crea un consumer para eventos de usuario.
func NewUserConsumer(service *application.UserService, batchSize int) *UserConsumer {
	return &UserConsumer{
		service:   service,
		batchSize: batchSize,
	}
}

// HandleMessage recibe un mensaje bruto y lo procesa.
func (c *UserConsumer) HandleMessage(ctx context.Context, key string, payload []byte) {
	var users []domain.User

	if err := json.Unmarshal(payload, &users); err != nil {
		log.Printf("⚠️ Failed to unmarshal user event payload: %v", err)
		return
	}

	for _, u := range users {
		ctxUser, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()

		existing, err := c.service.GetUser(ctxUser, u.ID)
		if err != nil {
			if err == domain.ErrUserNotFound {
				_, err = c.service.CreateUser(ctxUser, u.Email, u.Nombre, u.BirthDate)
				if err != nil {
					log.Printf("⚠️ Failed to create user %s: %v", u.ID, err)
				} else {
					log.Printf("✅ User created: %s", u.ID)
				}
			} else {
				log.Printf("⚠️ Error fetching user %s: %v", u.ID, err)
			}
		} else {
			existing.Email = u.Email
			existing.Nombre = u.Nombre
			existing.BirthDate = u.BirthDate

			if err := c.service.UpdateUser(ctxUser, existing); err != nil {
				log.Printf("⚠️ Failed to update user %s: %v", u.ID, err)
			} else {
				log.Printf("✅ User updated: %s", u.ID)
			}
		}
	}
}

// BackgroundConsumerChan levanta el consumer escuchando un canal de eventos
func BackgroundConsumerChan(ctx context.Context, ch <-chan UserEvent, consumer *UserConsumer) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Println("🛑 UserConsumer stopped")
				return
			case evt := <-ch:
				consumer.HandleMessage(ctx, evt.Key, evt.Payload)
			}
		}
	}()
}
