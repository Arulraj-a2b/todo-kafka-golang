package routes

import (
	"database/sql"
	"os"
	"todo-service/handlers"
	"todo-service/middleware"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func SetupRouter(db *sql.DB) *gin.Engine {
	router := gin.Default()

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	auth := router.Group("/", middleware.JWT())
	auth.GET("/todos", func(c *gin.Context) { handlers.GetTodos(c, db) })
	auth.POST("/todos", handlers.CreateTodo)
	auth.PUT("/todos/:id", handlers.UpdateTodo)
	auth.DELETE("/todos/:id", handlers.DeleteTodo)
	auth.GET("/todos/export", func(c *gin.Context) { handlers.ExportTodos(c, db) })
	auth.POST("/todos/import", handlers.ImportTodos)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8001"
	}
	router.Run("localhost:" + port)
	return router
}
