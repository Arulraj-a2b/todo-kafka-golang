package routes

import (
	"database/sql"
	"log/slog"
	"os"

	"todo-service/cache"
	"todo-service/handlers"
	"todo-service/internal/obs"
	"todo-service/middleware"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

func SetupRouter(db *sql.DB, cc *cache.Client) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(otelgin.Middleware("todo-service"))
	router.Use(obs.MetricsMiddleware())
	router.Use(obs.RequestLogger())

	obs.MountMetrics(router)

	// Swagger UI lives at the gateway (single source of truth).
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
	addr := "0.0.0.0:" + port
	slog.Info("todo-service listening", "addr", addr)
	if err := router.Run(addr); err != nil {
		slog.Error("server exit", "err", err)
		os.Exit(1)
	}
	return router
}
