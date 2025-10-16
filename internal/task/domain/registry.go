package domain

import (
	"reflect"

	sharedEvents "github.com/davicafu/hexagolab/shared/events"
)

// Las constantes de los tipos de evento se definen aquí, como valores string.
const (
	TaskCreated = "task.created"
	TaskUpdated = "task.updated"
	TaskDeleted = "task.deleted"
)

const TaskTopic = "task"

func NewEventRegistry() map[string]sharedEvents.EventMetadata {
	return map[string]sharedEvents.EventMetadata{
		TaskCreated: {
			Type:  reflect.TypeOf(Task{}),
			Topic: TaskTopic,
		},
		TaskUpdated: {
			Type:  reflect.TypeOf(Task{}),
			Topic: TaskTopic,
		},
		TaskDeleted: {
			Type:  reflect.TypeOf(Task{}),
			Topic: TaskTopic,
		},
	}
}
