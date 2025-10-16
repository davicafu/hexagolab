package application

import (
	"context"
	"errors"
	"time"

	userDomain "github.com/davicafu/hexagolab/internal/user/domain"
	sharedDomain "github.com/davicafu/hexagolab/shared/domain"
	sharedCache "github.com/davicafu/hexagolab/shared/platform/cache"
	sharedQuery "github.com/davicafu/hexagolab/shared/platform/query"
	sharedUtils "github.com/davicafu/hexagolab/shared/utils"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// UserService define los casos de uso relacionados con User.
type UserService struct {
	repo  userDomain.UserRepository
	cache sharedCache.Cache
	log   *zap.Logger
}

// NewUserService constructor
func NewUserService(repo userDomain.UserRepository, cache sharedCache.Cache, log *zap.Logger) *UserService {
	return &UserService{
		repo:  repo,
		cache: cache,
		log:   log,
	}
}

func (s *UserService) CreateUser(ctx context.Context, email, nombre string, birthDate time.Time) (*userDomain.User, error) {
	user := &userDomain.User{
		ID:        uuid.New(),
		Email:     email,
		Nombre:    nombre,
		BirthDate: birthDate,
		CreatedAt: time.Now().UTC(),
	}

	outboxEvent := sharedDomain.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "user",
		AggregateID:   user.ID.String(),
		EventType:     userDomain.UserCreated,
		Payload:       user,
		CreatedAt:     time.Now().UTC(),
		Processed:     false,
	}

	if err := s.repo.Create(ctx, user, outboxEvent); err != nil {
		return nil, err
	}

	sharedCache.AsyncCacheSet(ctx, s.cache, userDomain.UserCacheKeyByID(user.ID), user, 60, s.log)

	return user, nil
}

func (s *UserService) UpdateUser(ctx context.Context, u *userDomain.User) error {
	evt := sharedDomain.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "user",
		AggregateID:   u.ID.String(),
		EventType:     userDomain.UserUpdated,
		Payload:       u,
		CreatedAt:     time.Now().UTC(),
	}

	if err := s.repo.Update(ctx, u, evt); err != nil {
		return err
	}

	sharedCache.AsyncCacheSet(ctx, s.cache, userDomain.UserCacheKeyByID(u.ID), u, 60, s.log)

	return nil
}

func (s *UserService) DeleteUser(ctx context.Context, id uuid.UUID) error {
	evt := sharedDomain.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "user",
		AggregateID:   id.String(),
		EventType:     userDomain.UserDeleted,
		Payload:       id,
		CreatedAt:     time.Now().UTC(),
		Processed:     false,
	}

	if err := s.repo.DeleteByID(ctx, id, evt); err != nil {
		return err
	}

	sharedCache.AsyncCacheDelete(ctx, s.cache, userDomain.UserCacheKeyByID(id), s.log)

	return nil
}

// GetUser obtiene un usuario (primero intenta desde cache).
func (s *UserService) GetUser(ctx context.Context, id uuid.UUID) (*userDomain.User, error) {
	// 1. Intentar cache
	if s.cache != nil {
		var u userDomain.User
		if ok, _ := s.cache.Get(ctx, userDomain.UserCacheKeyByID(id), &u); ok {
			return &u, nil
		}
	}

	// 2. Ir al repo con reintentos
	var user *userDomain.User
	err := sharedUtils.Retry(ctx, 3, 100*time.Millisecond, func() error {
		var err error
		user, err = s.repo.GetByID(ctx, id)
		return err
	})
	if err != nil {
		if errors.Is(err, userDomain.ErrUserNotFound) {
			s.log.Warn("User not found", zap.String("user_id", id.String()))
		} else {
			s.log.Error("Failed to fetch user", zap.String("user_id", id.String()), zap.Error(err))
		}
		return nil, err
	}

	// 3. Actualizar cache en background sin bloquear la respuesta
	if s.cache != nil {
		go func(u *userDomain.User) {
			ctxCache, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			if err := s.cache.Set(ctxCache, userDomain.UserCacheKeyByID(u.ID), u, 60); err != nil {
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
func (s *UserService) ListUsers(ctx context.Context, criteria sharedDomain.Criteria, pagination sharedQuery.Pagination, sort sharedQuery.Sort) ([]*userDomain.User, error) {
	return s.repo.ListByCriteria(ctx, criteria, pagination, sort)
}

func (s *UserService) ListAdultUsers(ctx context.Context, pagination sharedQuery.Pagination, sort sharedQuery.Sort) ([]*userDomain.User, error) {
	minAge := 18
	criteria := sharedDomain.CompositeCriteria{
		Operator: sharedDomain.OpAnd,
		Criterias: []sharedDomain.Criteria{
			userDomain.AgeRangeCriteria{Min: &minAge},
		},
	}
	return s.repo.ListByCriteria(ctx, criteria, pagination, sort)
}
