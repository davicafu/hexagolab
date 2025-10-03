package http

import "github.com/gin-gonic/gin"

func RegisterUserRoutes(r *gin.Engine, handler *UserHandler) {
	users := r.Group("/users")
	{
		users.POST("/", handler.CreateUser)
		users.GET("/", handler.ListUsers)  // Listado de usuarios
		users.GET("/:id", handler.GetUser) // Usuario por id
		users.PUT("/:id", handler.UpdateUser)
		users.DELETE("/:id", handler.DeleteUser)
	}
}
