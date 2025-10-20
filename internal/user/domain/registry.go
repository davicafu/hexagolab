package domain

import (
	"reflect"

	sharedEvents "github.com/davicafu/hexagolab/internal/shared/domain/events"
)

// Las constantes de los tipos de evento se definen aquí, como valores string.
const (
	UserCreated = "user.created"
	UserUpdated = "user.updated"
	UserDeleted = "user.deleted"
)

const UserTopic = "user"

func NewEventRegistry() map[string]sharedEvents.EventMetadata {
	return map[string]sharedEvents.EventMetadata{
		UserCreated: {
			Type:  reflect.TypeOf(User{}),
			Topic: UserTopic,
		},
		UserUpdated: {
			Type:  reflect.TypeOf(User{}),
			Topic: UserTopic,
		},
		UserDeleted: {
			Type:  reflect.TypeOf(User{}),
			Topic: UserTopic,
		},
	}
}
