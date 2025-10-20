package bus

import "context"

type Keyer interface {
	PartitionKey() string
}

// La semántica de topic/nombre y formato del payload la decides en los adapters.
type EventBus interface {
	Publish(ctx context.Context, event interface{}) error
}
