package bus

import "context"

type Keyer interface {
	PartitionKey() string
}

// La semántica de topic/nombre y formato del payload la decides en los adapters.
type EventPublisher interface {
	Publish(ctx context.Context, event interface{}) error
}
