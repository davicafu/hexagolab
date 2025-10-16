package events

import (
	"context"
	"encoding/json"
	"sync"

	sharedBus "github.com/davicafu/hexagolab/shared/platform/bus"
)

// InMemoryEventBus implementa un bus de eventos para UN solo topic.
type InMemoryEventBus struct {
	subscribers []chan interface{} // <<-- AHORA ES UN SLICE, NO UN MAPA
	mu          sync.RWMutex
	stop        chan struct{}
	once        sync.Once
	topic       string // Identificador del topic que maneja este bus
}

// Verifica en tiempo de compilación que cumple la interfaz
var _ sharedBus.EventPublisher = (*InMemoryEventBus)(nil)

// NewInMemoryEventBus crea un bus de eventos para un topic específico.
func NewInMemoryEventBus(topic string) *InMemoryEventBus {
	return &InMemoryEventBus{
		subscribers: make([]chan interface{}, 0), // Inicializa el slice
		stop:        make(chan struct{}),
		topic:       topic,
	}
}

// Publish envía un evento a todos los suscriptores de este bus.
func (b *InMemoryEventBus) Publish(ctx context.Context, event interface{}) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	payloadBytes, err := json.Marshal(event)
	if err != nil {
		return err
	}

	if len(b.subscribers) > 0 {
		go b.distribute(b.subscribers, payloadBytes)
	}
	return nil
}

// distribute no necesita cambios.
func (b *InMemoryEventBus) distribute(subs []chan interface{}, event interface{}) {
	for _, subChan := range subs {
		select {
		case subChan <- event:
		default:
		}
	}
}

// Subscribe suscribe un nuevo oyente a este bus.
// Ya no necesita el parámetro 'bufferSize' si no se va a configurar dinámicamente.
func (b *InMemoryEventBus) Subscribe(bufferSize int) <-chan interface{} {
	b.mu.Lock()
	defer b.mu.Unlock()

	subChan := make(chan interface{}, bufferSize)
	// Añade el nuevo canal directamente al slice.
	b.subscribers = append(b.subscribers, subChan)
	return subChan
}
