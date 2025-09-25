package mocks

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"github.com/davicafu/hexagolab/internal/user/domain"
	"github.com/google/uuid"
)

// InMemoryUserRepo simula UserRepository con outbox incluido.
type InMemoryUserRepo struct {
	Users  map[uuid.UUID]*domain.User
	Outbox []domain.OutboxEvent
	mu     sync.Mutex
}

func NewInMemoryUserRepo() *InMemoryUserRepo {
	return &InMemoryUserRepo{
		Users:  make(map[uuid.UUID]*domain.User),
		Outbox: []domain.OutboxEvent{},
	}
}

// Create con outbox
func (r *InMemoryUserRepo) Create(ctx context.Context, u *domain.User, evt domain.OutboxEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.Users[u.ID]; ok {
		return domain.ErrUserAlreadyExists
	}
	r.Users[u.ID] = u
	r.Outbox = append(r.Outbox, evt)
	return nil
}

// GetByID
func (r *InMemoryUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.Users[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	return u, nil
}

// Update con outbox
func (r *InMemoryUserRepo) Update(ctx context.Context, u *domain.User, evt domain.OutboxEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.Users[u.ID]; !ok {
		return domain.ErrUserNotFound
	}
	r.Users[u.ID] = u
	r.Outbox = append(r.Outbox, evt)
	return nil
}

// DeleteByID con outbox
func (r *InMemoryUserRepo) DeleteByID(ctx context.Context, id uuid.UUID, evt domain.OutboxEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.Users[id]; !ok {
		return domain.ErrUserNotFound
	}
	delete(r.Users, id)
	r.Outbox = append(r.Outbox, evt)
	return nil
}

func (r *InMemoryUserRepo) List(ctx context.Context, f domain.UserFilter) ([]*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var list []*domain.User
	for _, u := range r.Users {
		if f.Nombre != nil && *f.Nombre != u.Nombre {
			continue
		}
		if f.Email != nil && *f.Email != u.Email {
			continue
		}

		list = append(list, u)
	}
	return list, nil
}

// ------------------- Outbox -------------------

// FetchPendingOutbox
func (r *InMemoryUserRepo) FetchPendingOutbox(ctx context.Context, limit int) ([]domain.OutboxEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if limit > len(r.Outbox) {
		limit = len(r.Outbox)
	}
	pending := r.Outbox[:limit]
	return append([]domain.OutboxEvent(nil), pending...), nil
}

// MarkOutboxProcessed
func (r *InMemoryUserRepo) MarkOutboxProcessed(ctx context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, evt := range r.Outbox {
		if evt.ID == id {
			// eliminar de outbox para simular que se procesó
			r.Outbox = append(r.Outbox[:i], r.Outbox[i+1:]...)
			return nil
		}
	}
	return domain.ErrUserNotFound
}

// ------------------- Cache -------------------
// DummyCache simula una cache en memoria
type DummyCache struct {
	store map[string]*domain.User
	mu    sync.Mutex
}

// NewDummyCache crea un DummyCache inicializado
func NewDummyCache() *DummyCache {
	return &DummyCache{
		store: make(map[string]*domain.User),
	}
}

func (c *DummyCache) SetForTest(key string, u *domain.User) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.store == nil {
		c.store = make(map[string]*domain.User)
	}
	c.store[key] = u
}

func (c *DummyCache) Get(ctx context.Context, key string, dest interface{}) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	u, ok := c.store[key]
	if !ok {
		return false, nil
	}

	userPtr, ok := dest.(*domain.User)
	if !ok {
		return false, nil
	}

	*userPtr = *u
	return true, nil
}

func (c *DummyCache) Set(ctx context.Context, key string, val interface{}, ttlSecs int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.store == nil {
		c.store = make(map[string]*domain.User)
	}

	u, ok := val.(*domain.User)
	if !ok {
		return nil
	}
	c.store[key] = u
	return nil
}

func (c *DummyCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.store, key)
	return nil
}

// ------------------- EventPublisher -------------------

type DummyPublisher struct {
	Published []string
	mu        sync.Mutex
}

func (p *DummyPublisher) Publish(ctx context.Context, topic string, event interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Guardar una versión JSON como evidencia
	data, _ := json.Marshal(event)
	p.Published = append(p.Published, string(data))
	log.Printf("Mock Published to %s: %s", topic, data)
	return nil
}
