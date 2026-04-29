package routes

import (
	"database/sql"
	"os"
	"todo-service/cache"
	"todo-service/handlers"
	"todo-service/middleware"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func SetupRouter(db *sql.DB, cc *cache.Client) *gin.Engine {
	router := gin.Default()

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	auth := router.Group("/", middleware.JWT())
	auth.GET("/todos", func(c *gin.Context) { handlers.GetTodos(c, db, cc) })
	auth.POST("/todos", func(c *gin.Context) { handlers.CreateTodo(c, db) })
	auth.PUT("/todos/:id", func(c *gin.Context) { handlers.UpdateTodo(c, db) })
	auth.DELETE("/todos/:id", func(c *gin.Context) { handlers.DeleteTodo(c, db) })
	auth.GET("/todos/export", func(c *gin.Context) { handlers.ExportTodos(c, db) })
	auth.POST("/todos/import", func(c *gin.Context) { handlers.ImportTodos(c, db) })

	port := os.Getenv("PORT")
	if port == "" {
		port = "8001"
	}
	router.Run("0.0.0.0:" + port)
	return router
}
