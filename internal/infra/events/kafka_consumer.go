package events

import (
	"context"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// MessageHandler define la interfaz que debe cumplir cualquier consumidor de eventos (como UserConsumer).
type MessageHandler interface {
	HandleMessage(ctx context.Context, key string, payload []byte)
}

// ConsumerAdapter es el "oído" que escucha en Kafka.
type ConsumerAdapter struct {
	reader  *kafka.Reader
	handler MessageHandler
	log     *zap.Logger
}

func NewConsumerAdapter(reader *kafka.Reader, handler MessageHandler, log *zap.Logger) *ConsumerAdapter {
	return &ConsumerAdapter{
		reader:  reader,
		handler: handler,
		log:     log,
	}
}

// Start inicia el bucle de consumo de mensajes en una goroutine.
func (c *ConsumerAdapter) Start(ctx context.Context) {
	c.log.Info("🎧 Iniciando consumidor de Kafka...",
		zap.String("topic", c.reader.Config().Topic),
		zap.Strings("brokers", c.reader.Config().Brokers),
	)

	go func() {
		for {
			// ReadMessage es una llamada bloqueante.
			msg, err := c.reader.ReadMessage(ctx)
			if err != nil {
				// Si el contexto se cancela, el error es normal y salimos limpiamente.
				if ctx.Err() != nil {
					c.log.Info("Consumidor de Kafka detenido.", zap.String("topic", c.reader.Config().Topic))
					return
				}
				c.log.Error("Error al leer mensaje de Kafka", zap.Error(err))
				continue // Continuamos con el siguiente mensaje
			}

			// Pasamos el mensaje al cerebro (UserConsumer) para que lo procese.
			c.handler.HandleMessage(ctx, string(msg.Key), msg.Value)
		}
	}()
}
