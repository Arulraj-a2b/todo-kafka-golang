package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"gateway-service/docs"
	"gateway-service/internal/obs"
	"gateway-service/middleware"
	"gateway-service/proxy"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

func main() {
	_ = godotenv.Load()

	obs.InitLogger("gateway-service")
	obs.InitMetrics()

	ctx := context.Background()
	shutdown := obs.InitTracer(ctx, "gateway-service")
	defer func() {
		shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdown(shCtx); err != nil {
			slog.Error("tracer shutdown", "err", err)
		}
	}()

	authURL := envOrDefault("AUTH_SERVICE_URL", "http://localhost:8000")
	todoURL := envOrDefault("TODO_SERVICE_URL", "http://localhost:8001")
	port := envOrDefault("GATEWAY_PORT", "8080")

	rdb := newRedis()
	limiter := middleware.NewRateLimiter(rdb)

	authProxy := proxy.New(authURL, "/api/auth")
	todoProxy := proxy.New(todoURL, "/api")

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(otelgin.Middleware("gateway-service"))
	router.Use(obs.MetricsMiddleware())
	router.Use(obs.RequestLogger())
	router.Use(middleware.CORS())

	obs.MountMetrics(router)

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Unified API docs hosted by the gateway (single source of truth across
	// all backend services). The OpenAPI spec is embedded at compile time.
	router.GET("/docs", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/docs/")
	})
	router.GET("/docs/", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(docs.SwaggerHTML))
	})
	router.GET("/docs/swagger.yaml", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/yaml", docs.SwaggerYAML)
	})

	api := router.Group("/api")

	authGroup := api.Group("/auth")
	authGroup.POST("/login", limiter.PerIP("login", 5, 60), authProxy)
	authGroup.POST("/register", limiter.PerIP("register", 3, 60), authProxy)
	authGroup.GET("/me", limiter.PerUser("me", 60, 60), authProxy)

	todoGroup := api.Group("/todos")
	todoGroup.GET("/export", limiter.PerUser("todos-export", 5, 3600), todoProxy)
	todoGroup.POST("/import", limiter.PerUser("todos-import", 5, 3600), todoProxy)
	todoGroup.GET("", limiter.PerUser("todos-read", 120, 60), todoProxy)
	todoGroup.POST("", limiter.PerUser("todos-write", 60, 60), todoProxy)
	todoGroup.PUT("/:id", limiter.PerUser("todos-write", 60, 60), todoProxy)
	todoGroup.DELETE("/:id", limiter.PerUser("todos-write", 60, 60), todoProxy)

	addr := "0.0.0.0:" + port
	slog.Info("gateway listening", "addr", addr, "auth", authURL, "todo", todoURL)
	if err := router.Run(addr); err != nil {
		slog.Error("server exit", "err", err)
		os.Exit(1)
	}
}

func newRedis() *redis.Client {
	url := strings.TrimSpace(os.Getenv("REDIS_URL"))
	if url == "" {
		slog.Info("REDIS_URL not set; rate limiting will fail-open")
		return nil
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		slog.Warn("invalid REDIS_URL; rate limiting will fail-open", "url", url, "err", err)
		return nil
	}
	c := redis.NewClient(opt)
	if err := c.Ping(context.Background()).Err(); err != nil {
		slog.Warn("redis unreachable; rate limiting will fail-open", "url", url, "err", err)
		return nil
	}
	slog.Info("redis connected", "url", url)
	return c
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
