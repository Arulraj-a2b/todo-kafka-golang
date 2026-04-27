package routes

import (
	"database/sql"
	"todo-kafka/handlers"
	"todo-kafka/middleware"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func SetupRouter(db *sql.DB) *gin.Engine {
	router := gin.Default()

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	router.POST("/register", handlers.Register(db))
	router.POST("/login", handlers.Login(db))

	auth := router.Group("/", middleware.JWT())
	auth.GET("/todos", func(c *gin.Context) { handlers.GetTodos(c, db) })
	auth.POST("/todos", handlers.CreateTodo)
	auth.PUT("/todos/:id", handlers.UpdateTodo)
	auth.DELETE("/todos/:id", handlers.DeleteTodo)
	auth.GET("/todos/export", func(c *gin.Context) { handlers.ExportTodos(c, db) })
	auth.POST("/todos/import", handlers.ImportTodos)

	router.Run("localhost:8000")
	return router
}
