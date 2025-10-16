// en internal/infra/web/http/task_handler.go
package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/davicafu/hexagolab/internal/task/application"
	taskDomain "github.com/davicafu/hexagolab/internal/task/domain"
	sharedDomain "github.com/davicafu/hexagolab/shared/domain"
	sharedQuery "github.com/davicafu/hexagolab/shared/platform/query"
)

// TaskHandler encapsula los endpoints HTTP relacionados con Task.
type TaskHandler struct {
	service *application.TaskService
}

// NewTaskHandler crea un nuevo TaskHandler.
func NewTaskHandler(service *application.TaskService) *TaskHandler {
	return &TaskHandler{service: service}
}

// --- Handlers CRUD ---

// CreateTask endpoint POST /tasks
func (h *TaskHandler) CreateTask(c *gin.Context) {
	var req struct {
		Title       string    `json:"title" binding:"required"`
		Description string    `json:"description"`
		AssigneeID  uuid.UUID `json:"assigneeId" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	task, err := h.service.CreateTask(c.Request.Context(), req.Title, req.Description, req.AssigneeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, task)
}

// GetTask endpoint GET /tasks/:id
func (h *TaskHandler) GetTask(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task id"})
		return
	}

	task, err := h.service.GetTaskByID(c.Request.Context(), id)
	if err != nil {
		if err == taskDomain.ErrTaskNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

// UpdateTask endpoint PUT /tasks/:id
func (h *TaskHandler) UpdateTask(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task id"})
		return
	}

	// Usamos punteros para que los campos sean opcionales en el JSON
	var req struct {
		Title       *string `json:"title,omitempty"`
		Description *string `json:"description,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	task, err := h.service.GetTaskByID(c.Request.Context(), id)
	if err != nil {
		// Manejar error de "no encontrado"
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}

	// Aplicamos los cambios si se proporcionaron
	if req.Title != nil {
		task.Title = *req.Title
	}
	if req.Description != nil {
		task.Description = *req.Description
	}

	// Llamamos al método Update del dominio
	task.Update(task.Title, task.Description)

	if err := h.service.UpdateTask(c.Request.Context(), task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

// DeleteTask endpoint DELETE /tasks/:id
func (h *TaskHandler) DeleteTask(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task id"})
		return
	}

	if err := h.service.DeleteTask(c.Request.Context(), id); err != nil {
		if err == taskDomain.ErrTaskNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// ListTasks endpoint GET /tasks con filtros, paginación y ordenamiento
func (h *TaskHandler) ListTasks(c *gin.Context) {
	var criterias []sharedDomain.Criteria

	// --- Filtros desde query params ---
	if title := c.Query("title"); title != "" {
		criterias = append(criterias, taskDomain.TitleLikeCriteria{Title: title})
	}
	if status := c.Query("status"); status != "" {
		criterias = append(criterias, taskDomain.StatusCriteria{Status: taskDomain.TaskStatus(status)})
	}
	if assigneeID := c.Query("assigneeId"); assigneeID != "" {
		if id, err := uuid.Parse(assigneeID); err == nil {
			criterias = append(criterias, taskDomain.AssigneeIDCriteria{ID: id})
		}
	}

	criteria := sharedDomain.And(criterias...)

	// --- Sort (lógica idéntica a la de User) ---
	sortParam := sharedQuery.Sort{Field: "created_at", Desc: true}
	if sortField := c.Query("sort_field"); sortField != "" {
		sortParam.Field = sortField
		sortParam.Desc = c.Query("sort_desc") == "true"
	}

	// --- Paginación (lógica idéntica a la de User) ---
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	pagination := sharedQuery.OffsetPagination{Limit: limit, Offset: offset}

	// --- Llamada al servicio ---
	tasks, err := h.service.ListTasks(c.Request.Context(), criteria, pagination, sortParam)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, tasks)
}
