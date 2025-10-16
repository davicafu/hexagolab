package http

import "github.com/gin-gonic/gin"

// RegisterTaskRoutes registra las rutas HTTP para el dominio de Tareas.
func RegisterTaskRoutes(r *gin.Engine, handler *TaskHandler) {
	// Agrupamos todas las rutas de tareas bajo el prefijo "/tasks"
	tasks := r.Group("/tasks")
	{
		tasks.POST("/", handler.CreateTask)      // Crear una nueva tarea
		tasks.GET("/", handler.ListTasks)        // Listar todas las tareas
		tasks.GET("/:id", handler.GetTask)       // Obtener una tarea por su ID
		tasks.PUT("/:id", handler.UpdateTask)    // Actualizar una tarea existente
		tasks.DELETE("/:id", handler.DeleteTask) // Eliminar una tarea
	}
}
