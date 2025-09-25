package events

import (
	"context"
	"encoding/json"
	"log"

	"github.com/segmentio/kafka-go"

	"github.com/davicafu/hexagolab/internal/user/domain"
)

type KafkaUserPublisher struct {
	writer *kafka.Writer
}

func NewKafkaUserPublisher(writer *kafka.Writer) *KafkaUserPublisher {
	return &KafkaUserPublisher{writer: writer}
}

func (p *KafkaUserPublisher) Publish(ctx context.Context, topic string, event interface{}) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	var key []byte
	switch e := event.(type) {
	case *domain.User:
		key = []byte(e.ID.String()) // usar ID del usuario como key
	default:
		key = nil
	}

	msg := kafka.Message{
		Topic: topic,
		Key:   key,
		Value: data,
	}

	err = p.writer.WriteMessages(ctx, msg)
	if err != nil {
		log.Printf("Error publishing to Kafka: %v", err)
		return err
	}
	return nil
}
