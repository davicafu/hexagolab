package contracts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/davicafu/hexagolab/internal/user/application"
	userDomain "github.com/davicafu/hexagolab/internal/user/domain"
	sharedDomain "github.com/davicafu/hexagolab/shared/domain"
	"github.com/davicafu/hexagolab/shared/events"
	"github.com/davicafu/hexagolab/tests/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// userHTTPResponse define el formato que esperamos en las respuestas JSON
type userHTTPResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Nombre    string `json:"nombre"`
	BirthDate string `json:"birth_date"`
	CreatedAt string `json:"created_at"`
}

func TestGetUser_HTTPContract(t *testing.T) {
	// Mock repository que simula un usuario existente
	repo := mocks.NewInMemoryUserRepo()
	cache := &mocks.DummyCache{}

	service := application.NewUserService(repo, cache, zap.NewNop())

	// Crear usuario de prueba
	userID := uuid.New()
	u := &userDomain.User{
		ID:        userID,
		Email:     "test@example.com",
		Nombre:    "Test User",
		BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
		CreatedAt: time.Now(),
	}
	assert.NoError(t, repo.Create(context.Background(), u, sharedDomain.OutboxEvent{}))

	// Crear handler HTTP simple
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parsear id desde query para simplificar
		id := r.URL.Query().Get("id")
		uid, _ := uuid.Parse(id)

		user, err := service.GetUser(r.Context(), uid)
		if err != nil {
			if err == userDomain.ErrUserNotFound {
				http.Error(w, "user not found", http.StatusNotFound)
				return
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(userHTTPResponse{
			ID:        user.ID.String(),
			Email:     user.Email,
			Nombre:    user.Nombre,
			BirthDate: user.BirthDate.Format("2006-01-02"),
			CreatedAt: user.CreatedAt.Format(time.RFC3339),
		})
	})

	// Test: usuario existente
	req := httptest.NewRequest(http.MethodGet, "/user?id="+userID.String(), nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp userHTTPResponse
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, u.ID.String(), resp.ID)
	assert.Equal(t, u.Email, resp.Email)
	assert.Equal(t, u.Nombre, resp.Nombre)
	assert.Equal(t, u.BirthDate.Format("2006-01-02"), resp.BirthDate)

	// Test: usuario no existente
	nonexistentID := uuid.New()
	req2 := httptest.NewRequest(http.MethodGet, "/user?id="+nonexistentID.String(), nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	assert.Equal(t, http.StatusNotFound, rec2.Code)
	assert.Contains(t, rec2.Body.String(), "user not found")
}

// --- MockPublisher captura eventos publicados ---
type MockPublisher struct {
	Published []events.IntegrationEvent
}

func (m *MockPublisher) Publish(ctx context.Context, eventType string, payload []byte) error {
	var ie events.IntegrationEvent
	if err := json.Unmarshal(payload, &ie); err != nil {
		return err
	}
	m.Published = append(m.Published, ie)
	return nil
}
