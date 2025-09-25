package http

import "github.com/gin-gonic/gin"

func RegisterUserRoutes(r *gin.Engine, handler *UserHandler) {
	users := r.Group("/users")
	{
		users.POST("/", handler.CreateUser)
		users.GET("/:id", handler.GetUser)
		users.GET("/", handler.ListUsers)
		users.PUT("/:id", handler.UpdateUser)
		users.DELETE("/:id", handler.DeleteUser)
	}
}
