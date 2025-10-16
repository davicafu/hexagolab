package mocks

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	userDomain "github.com/davicafu/hexagolab/internal/user/domain"
	sharedDomain "github.com/davicafu/hexagolab/shared/domain"
	sharedQuery "github.com/davicafu/hexagolab/shared/platform/query"
	"github.com/google/uuid"
)

// InMemoryUserRepo simula UserRepository con outbox incluido.
type InMemoryUserRepo struct {
	Users  map[uuid.UUID]*userDomain.User
	Outbox []sharedDomain.OutboxEvent
	mu     sync.Mutex
}

func NewInMemoryUserRepo() *InMemoryUserRepo {
	return &InMemoryUserRepo{
		Users:  make(map[uuid.UUID]*userDomain.User),
		Outbox: []sharedDomain.OutboxEvent{},
	}
}

// Create con outbox
func (r *InMemoryUserRepo) Create(ctx context.Context, u *userDomain.User, evt sharedDomain.OutboxEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.Users[u.ID]; ok {
		return userDomain.ErrUserAlreadyExists
	}
	r.Users[u.ID] = u
	r.Outbox = append(r.Outbox, evt)
	return nil
}

// GetByID
func (r *InMemoryUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*userDomain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.Users[id]
	if !ok {
		return nil, userDomain.ErrUserNotFound
	}
	return u, nil
}

// Update con outbox
func (r *InMemoryUserRepo) Update(ctx context.Context, u *userDomain.User, evt sharedDomain.OutboxEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.Users[u.ID]; !ok {
		return userDomain.ErrUserNotFound
	}
	r.Users[u.ID] = u
	r.Outbox = append(r.Outbox, evt)
	return nil
}

// DeleteByID con outbox
func (r *InMemoryUserRepo) DeleteByID(ctx context.Context, id uuid.UUID, evt sharedDomain.OutboxEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.Users[id]; !ok {
		return userDomain.ErrUserNotFound
	}
	delete(r.Users, id)
	r.Outbox = append(r.Outbox, evt)
	return nil
}

// ListByCriteria en el mock (mocks package)
func (r *InMemoryUserRepo) ListByCriteria(
	ctx context.Context,
	criteria sharedDomain.Criteria,
	pagination sharedQuery.Pagination,
	s sharedQuery.Sort, // renombrado para no colisionar con package sort
) ([]*userDomain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var list []*userDomain.User
	for _, u := range r.Users {
		// Si no hay criterio, consideramos que coincide todo
		if criteria == nil {
			list = append(list, u)
			continue
		}

		conds := criteria.ToConditions()
		matchesAll := true
		for _, cond := range conds {
			if !matchCriterion(u, cond) {
				matchesAll = false
				break
			}
		}
		if matchesAll {
			list = append(list, u)
		}
	}

	// ordenar
	if s.Field != "" {
		switch s.Field {
		case "nombre":
			sort.Slice(list, func(i, j int) bool {
				if s.Desc {
					return list[i].Nombre > list[j].Nombre
				}
				return list[i].Nombre < list[j].Nombre
			})
		case "email":
			sort.Slice(list, func(i, j int) bool {
				if s.Desc {
					return list[i].Email > list[j].Email
				}
				return list[i].Email < list[j].Email
			})
		case "created_at":
			sort.Slice(list, func(i, j int) bool {
				if s.Desc {
					return list[i].CreatedAt.After(list[j].CreatedAt)
				}
				return list[i].CreatedAt.Before(list[j].CreatedAt)
			})
		}
	}

	// paginación
	switch p := pagination.(type) {
	case sharedQuery.OffsetPagination:
		start := p.Offset
		if start > len(list) {
			return []*userDomain.User{}, nil
		}
		end := start + p.Limit
		if end > len(list) {
			end = len(list)
		}
		return list[start:end], nil

	case sharedQuery.CursorPagination:
		// 1️⃣ Ordenar la lista según sortField y SortDesc, usando ID como tie-breaker
		sort.SliceStable(list, func(i, j int) bool {
			var vi, vj string
			switch p.SortField {
			case "created_at":
				vi = list[i].CreatedAt.Format(time.RFC3339Nano)
				vj = list[j].CreatedAt.Format(time.RFC3339Nano)
			case "nombre":
				vi = list[i].Nombre
				vj = list[j].Nombre
			case "email":
				vi = list[i].Email
				vj = list[j].Email
			default:
				vi = list[i].ID.String()
				vj = list[j].ID.String()
			}

			if p.SortDesc {
				if vi != vj {
					return vi > vj
				}
				return list[i].ID.String() > list[j].ID.String()
			}
			if vi != vj {
				return vi < vj
			}
			return list[i].ID.String() < list[j].ID.String()
		})

		// 2️⃣ Filtrar según cursor compuesto
		filtered := []*userDomain.User{}
		startCollect := p.Cursor == ""
		var cursorSort, cursorID string
		if p.Cursor != "" {
			parts := strings.SplitN(p.Cursor, "|", 2)
			cursorSort = parts[0]
			cursorID = parts[1]
		}

		for _, u := range list {
			if !startCollect {
				uSort := ""
				switch p.SortField {
				case "created_at":
					uSort = u.CreatedAt.Format(time.RFC3339Nano)
				case "nombre":
					uSort = u.Nombre
				case "email":
					uSort = u.Email
				default:
					uSort = u.ID.String()
				}

				if !p.SortDesc {
					if uSort > cursorSort || (uSort == cursorSort && u.ID.String() > cursorID) {
						startCollect = true
					}
				} else {
					if uSort < cursorSort || (uSort == cursorSort && u.ID.String() < cursorID) {
						startCollect = true
					}
				}
				if !startCollect {
					continue
				}
			}

			filtered = append(filtered, u)
			if len(filtered) >= p.Limit {
				break
			}
		}

		return filtered, nil

	default:
		// si no se reconoce, devolvemos todo (sin paginar)
		return list, nil
	}
}

// matchCriterion evalúa un domain.Criterion contra un usuario en memoria.
func matchCriterion(u *userDomain.User, crit sharedDomain.Criterion) bool {
	op := strings.ToUpper(strings.TrimSpace(string(crit.Op)))
	field := crit.Field

	switch field {
	case "id":
		// puede venir como uuid.UUID o como string
		switch v := crit.Value.(type) {
		case uuid.UUID:
			return u.ID == v
		case string:
			return u.ID.String() == v
		default:
			// intentar comparar por formato string
			return u.ID.String() == fmt.Sprintf("%v", crit.Value)
		}

	case "email":
		val := fmt.Sprintf("%v", crit.Value)
		if op == "ILIKE" || op == "LIKE" {
			// pattern esperado con %...% -> hacer Contains
			p := strings.Trim(val, "%")
			if op == "ILIKE" {
				return strings.Contains(strings.ToLower(u.Email), strings.ToLower(p))
			}
			return strings.Contains(u.Email, p)
		}
		// igualdad simple
		return u.Email == val

	case "nombre":
		val := fmt.Sprintf("%v", crit.Value)
		if op == "ILIKE" || op == "LIKE" {
			p := strings.Trim(val, "%")
			if op == "ILIKE" {
				return strings.Contains(strings.ToLower(u.Nombre), strings.ToLower(p))
			}
			return strings.Contains(u.Nombre, p)
		}
		return u.Nombre == val

	case "birth_date", "birthdate":
		// Value esperado time.Time
		valTime, ok := crit.Value.(time.Time)
		if !ok {
			// intentar parsear si viene como string
			if s, ok2 := crit.Value.(string); ok2 {
				t, err := time.Parse(time.RFC3339, s)
				if err == nil {
					valTime = t
					ok = true
				}
			}
		}
		if !ok {
			// no sabemos comparar -> asumir que coincide
			return true
		}

		switch op {
		case "<", "<=":
			if u.BirthDate.Before(valTime) || u.BirthDate.Equal(valTime) {
				return true
			}
			return false
		case ">", ">=":
			if u.BirthDate.After(valTime) || u.BirthDate.Equal(valTime) {
				return true
			}
			return false
		case "=":
			return u.BirthDate.Equal(valTime)
		default:
			return true
		}

	default:
		// criterio desconocido: no filtrar (mejor ser permisivo en mock)
		return true
	}
}

// ------------------- Outbox -------------------

// FetchPendingOutbox
func (r *InMemoryUserRepo) FetchPendingOutbox(ctx context.Context, limit int) ([]sharedDomain.OutboxEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if limit > len(r.Outbox) {
		limit = len(r.Outbox)
	}
	pending := r.Outbox[:limit]
	return append([]sharedDomain.OutboxEvent(nil), pending...), nil
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
	return userDomain.ErrUserNotFound
}

// ------------------- Cache -------------------
// DummyCache simula una cache en memoria
type DummyCache struct {
	store map[string]*userDomain.User
	mu    sync.Mutex
}

// NewDummyCache crea un DummyCache inicializado
func NewDummyCache() *DummyCache {
	return &DummyCache{
		store: make(map[string]*userDomain.User),
	}
}

func (c *DummyCache) SetForTest(key string, u *userDomain.User) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.store == nil {
		c.store = make(map[string]*userDomain.User)
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

	userPtr, ok := dest.(*userDomain.User)
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
		c.store = make(map[string]*userDomain.User)
	}

	u, ok := val.(*userDomain.User)
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
