package http

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/davicafu/hexagolab/internal/user/application"
	"github.com/davicafu/hexagolab/internal/user/domain"
)

// UserHandler encapsula los endpoints HTTP relacionados con User
type UserHandler struct {
	service *application.UserService
}

// NewUserHandler crea un nuevo UserHandler
func NewUserHandler(service *application.UserService) *UserHandler {
	return &UserHandler{service: service}
}

// ---------------- Handlers ----------------

// CreateUser endpoint POST /users
func (h *UserHandler) CreateUser(c *gin.Context) {
	var req struct {
		Email     string `json:"email" binding:"required,email"`
		Nombre    string `json:"nombre" binding:"required"`
		BirthDate string `json:"birth_date" binding:"required"` // ISO8601, ej: 2000-01-01
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	birthDate, err := time.Parse("2006-01-02", req.BirthDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid birth_date format, use YYYY-MM-DD"})
		return
	}

	user, err := h.service.CreateUser(c.Request.Context(), req.Email, req.Nombre, birthDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, user)
}

// GetUser endpoint GET /users/:id
func (h *UserHandler) GetUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	user, err := h.service.GetUser(c.Request.Context(), id)
	if err != nil {
		if err == domain.ErrUserNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, user)
}

// UpdateUser endpoint PUT /users/:id
func (h *UserHandler) UpdateUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	var req struct {
		Email     *string `json:"email,omitempty"`
		Nombre    *string `json:"nombre,omitempty"`
		BirthDate *string `json:"birth_date,omitempty"` // ISO8601
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.service.GetUser(c.Request.Context(), id)
	if err != nil {
		if err == domain.ErrUserNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if req.Email != nil {
		user.Email = *req.Email
	}
	if req.Nombre != nil {
		user.Nombre = *req.Nombre
	}
	if req.BirthDate != nil {
		bd, err := time.Parse("2006-01-02", *req.BirthDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid birth_date format"})
			return
		}
		user.BirthDate = bd
	}

	if err := h.service.UpdateUser(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, user)
}

// DeleteUser endpoint DELETE /users/:id
func (h *UserHandler) DeleteUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	if err := h.service.DeleteUser(c.Request.Context(), id); err != nil {
		if err == domain.ErrUserNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *UserHandler) ListUsers(c *gin.Context) {
	var criterias []domain.Criteria

	// --- Filtros desde query params ---
	if nombre := c.Query("nombre"); nombre != "" {
		criterias = append(criterias, domain.NameLikeCriteria{Name: nombre})
	}

	if email := c.Query("email"); email != "" {
		criterias = append(criterias, domain.EmailCriteria{Email: email})
	}

	if idStr := c.Query("id"); idStr != "" {
		if id, err := uuid.Parse(idStr); err == nil {
			criterias = append(criterias, domain.IDCriteria{ID: id})
		}
	}

	// Edad
	var min, max *int
	if minAge := c.Query("min_age"); minAge != "" {
		if v, err := strconv.Atoi(minAge); err == nil {
			min = &v
		}
	}
	if maxAge := c.Query("max_age"); maxAge != "" {
		if v, err := strconv.Atoi(maxAge); err == nil {
			max = &v
		}
	}
	if min != nil || max != nil {
		criterias = append(criterias, domain.AgeRangeCriteria{Min: min, Max: max})
	}

	criteria := domain.CompositeCriteria{
		Operator:  domain.OpAnd,
		Criterias: criterias,
	}

	// --- Sort ---
	sortParam := domain.Sort{
		Field: "created_at",
		Desc:  true,
	}
	if sortField := c.Query("sort_field"); sortField != "" {
		sortParam.Field = sortField
		if sortDesc := c.Query("sort_desc"); sortDesc == "true" {
			sortParam.Desc = true
		}
	}

	// --- Paginación ---
	var pagination domain.Pagination
	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil {
			limit = v
		}
	}

	if cursor := c.Query("cursor"); cursor != "" {
		pagination = domain.CursorPagination{
			Limit:     limit,
			Cursor:    cursor,
			SortField: sortParam.Field,
			SortDesc:  sortParam.Desc,
		}
	} else {
		offset := 0
		if offsetStr := c.Query("offset"); offsetStr != "" {
			if v, err := strconv.Atoi(offsetStr); err == nil {
				offset = v
			}
		}
		pagination = domain.OffsetPagination{
			Limit:  limit,
			Offset: offset,
		}
	}

	users, err := h.service.ListUsers(c.Request.Context(), criteria, pagination, sortParam)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, users)
}
