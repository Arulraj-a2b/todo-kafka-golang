package routes

import (
	"database/sql"
	"log/slog"
	"os"

	"auth-service/cache"
	"auth-service/handlers"
	"auth-service/internal/obs"
	"auth-service/middleware"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

func SetupRouter(db *sql.DB, cc *cache.Client) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	// otelgin starts a span per request and stores it on the request context.
	// MUST be registered before any middleware that wants the span (logger,
	// metrics) so they can attach to it.
	router.Use(otelgin.Middleware("auth-service"))
	router.Use(obs.MetricsMiddleware())
	router.Use(obs.RequestLogger())

	obs.MountMetrics(router)

	// Swagger UI lives at the gateway (single source of truth).
	router.POST("/register", handlers.Register(db))
	router.POST("/login", handlers.Login(db, cc))

	auth := router.Group("/", middleware.JWT())
	auth.GET("/me", handlers.Me(db, cc))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}
	addr := "0.0.0.0:" + port
	slog.Info("auth-service listening", "addr", addr)
	if err := router.Run(addr); err != nil {
		slog.Error("server exit", "err", err)
		os.Exit(1)
	}
	return router
}
