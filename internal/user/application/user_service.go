package application

import (
	"context"
	"errors"
	"time"

	"github.com/davicafu/hexagolab/internal/user/domain"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// UserService define los casos de uso relacionados con User.
type UserService struct {
	repo   domain.UserRepository
	cache  domain.UserCache
	events domain.EventPublisher
	log    *zap.Logger
}

// NewUserService constructor
func NewUserService(repo domain.UserRepository, cache domain.UserCache, events domain.EventPublisher, log *zap.Logger) *UserService {
	return &UserService{
		repo:   repo,
		cache:  cache,
		events: events,
		log:    log,
	}
}

func retry(ctx context.Context, attempts int, delay time.Duration, fn func() error) error {
	var err error
	for i := 0; i < attempts; i++ {
		err = fn()
		if err == nil {
			return nil
		}

		select {
		case <-time.After(delay):
			// espera antes del siguiente intento
		case <-ctx.Done():
			return ctx.Err() // contexto cancelado
		}
	}
	return err
}

func (s *UserService) CreateUser(ctx context.Context, email, nombre string, birthDate time.Time) (*domain.User, error) {
	user := &domain.User{
		ID:        uuid.New(),
		Email:     email,
		Nombre:    nombre,
		BirthDate: birthDate,
		CreatedAt: time.Now().UTC(),
	}

	outboxEvent := domain.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "user",
		AggregateID:   user.ID.String(),
		EventType:     "user.created",
		Payload:       user,
		CreatedAt:     time.Now().UTC(),
		Processed:     false,
	}

	if err := s.repo.Create(ctx, user, outboxEvent); err != nil {
		return nil, err
	}

	if s.cache != nil {
		go func(u *domain.User) {
			ctxCache, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			_ = s.cache.Set(ctxCache, domain.CacheKeyByID(u.ID), u, 60)
		}(user)
	}

	return user, nil
}

func (s *UserService) UpdateUser(ctx context.Context, u *domain.User) error {
	evt := domain.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "user",
		AggregateID:   u.ID.String(),
		EventType:     "user.updated",
		Payload:       u,
		CreatedAt:     time.Now().UTC(),
	}

	if err := s.repo.Update(ctx, u, evt); err != nil {
		return err
	}

	if s.cache != nil {
		go func() { _ = s.cache.Set(ctx, domain.CacheKeyByID(u.ID), u, 60) }()
	}

	return nil
}

func (s *UserService) DeleteUser(ctx context.Context, id uuid.UUID) error {
	evt := domain.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "user",
		AggregateID:   id.String(),
		EventType:     "user.deleted",
		Payload:       id,
		CreatedAt:     time.Now().UTC(),
		Processed:     false,
	}

	if err := s.repo.DeleteByID(ctx, id, evt); err != nil {
		return err
	}

	if s.cache != nil {
		go func(uid uuid.UUID) { _ = s.cache.Delete(ctx, domain.CacheKeyByID(uid)) }(id)
	}

	return nil
}

// GetUser obtiene un usuario (primero intenta desde cache).
func (s *UserService) GetUser(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	// 1. Intentar cache
	if s.cache != nil {
		var u domain.User
		if ok, _ := s.cache.Get(ctx, domain.CacheKeyByID(id), &u); ok {
			return &u, nil
		}
	}

	// 2. Ir al repo con reintentos
	var user *domain.User
	err := retry(ctx, 3, 100*time.Millisecond, func() error {
		var err error
		user, err = s.repo.GetByID(ctx, id)
		return err
	})
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			s.log.Warn("User not found", zap.String("user_id", id.String()))
		} else {
			s.log.Error("Failed to fetch user", zap.String("user_id", id.String()), zap.Error(err))
		}
		return nil, err
	}

	// 3. Actualizar cache en background sin bloquear la respuesta
	if s.cache != nil {
		go func(u *domain.User) {
			ctxCache, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			if err := s.cache.Set(ctxCache, domain.CacheKeyByID(u.ID), u, 60); err != nil {
				s.log.Warn("⚠️ Cache update failed",
					zap.String("user_id", u.ID.String()),
					zap.Error(err),
				)
			}
		}(user)
	}

	return user, nil
}

// ListUsers devuelve todos los usuarios aplicando filtros.
func (s *UserService) ListUsers(ctx context.Context, criteria domain.Criteria, pagination domain.Pagination, sort domain.Sort) ([]*domain.User, error) {
	return s.repo.ListByCriteria(ctx, criteria, pagination, sort)
}

func (s *UserService) ListAdultUsers(ctx context.Context, pagination domain.Pagination, sort domain.Sort) ([]*domain.User, error) {
	minAge := 18
	criteria := domain.CompositeCriteria{
		Operator: domain.OpAnd,
		Criterias: []domain.Criteria{
			domain.AgeRangeCriteria{Min: &minAge},
		},
	}
	return s.repo.ListByCriteria(ctx, criteria, pagination, sort)
}
