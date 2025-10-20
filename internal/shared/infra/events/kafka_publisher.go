package events

import (
	"context"
	"encoding/json"

	"go.uber.org/zap"

	"github.com/segmentio/kafka-go"

	sharedBus "github.com/davicafu/hexagolab/internal/shared/infra/platform/bus"
)

type KafkaPublisher struct {
	writer *kafka.Writer
	log    *zap.Logger
}

func NewKafkaPublisher(writer *kafka.Writer, log *zap.Logger) *KafkaPublisher {
	return &KafkaPublisher{writer: writer, log: log}
}

func (p *KafkaPublisher) Publish(ctx context.Context, event interface{}) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	var key []byte
	if keyer, ok := event.(sharedBus.Keyer); ok {
		key = []byte(keyer.PartitionKey())
	}

	msg := kafka.Message{
		Key:   key,
		Value: data,
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		p.log.Error("Error publishing to Kafka", zap.Error(err))
		return err
	}

	p.log.Debug("Event published successfully", zap.Any("event", event))
	return nil
}

// Verificación estática
var _ sharedBus.EventBus = (*KafkaPublisher)(nil)
