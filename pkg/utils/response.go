// en pkg/utils/response.go
package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ErrorResponse define la estructura estándar para las respuestas de error.
type ErrorResponse struct {
	Message string `json:"message"`
	// Code    string `json:"code,omitempty"` // Opcional: un código de error interno
}

// SendSuccess envía una respuesta exitosa con un payload de datos.
func SendSuccess(c *gin.Context, statusCode int, data interface{}) {
	c.JSON(statusCode, gin.H{
		"data": data,
	})
}

// SendError envía una respuesta de error con un formato estandarizado.
func SendError(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, gin.H{
		"error": ErrorResponse{
			Message: message,
		},
	})
}

// --- Helpers específicos para errores comunes ---

func SendBadRequest(c *gin.Context, message string) {
	SendError(c, http.StatusBadRequest, message)
}

func SendNotFound(c *gin.Context, message string) {
	SendError(c, http.StatusNotFound, message)
}

func SendInternalServerError(c *gin.Context, message string) {
	SendError(c, http.StatusInternalServerError, message)
}
