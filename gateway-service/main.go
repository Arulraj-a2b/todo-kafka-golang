package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"gateway-service/middleware"
	"gateway-service/proxy"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	authURL := envOrDefault("AUTH_SERVICE_URL", "http://localhost:8000")
	todoURL := envOrDefault("TODO_SERVICE_URL", "http://localhost:8001")
	port := envOrDefault("GATEWAY_PORT", "8080")

	rdb := newRedis()
	limiter := middleware.NewRateLimiter(rdb)

	authProxy := proxy.New(authURL, "/api/auth")
	todoProxy := proxy.New(todoURL, "/api")

	router := gin.Default()
	router.Use(middleware.CORS())

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := router.Group("/api")

	// /api/auth — explicit endpoints with tiered limits.
	authGroup := api.Group("/auth")
	authGroup.POST("/login", limiter.PerIP("login", 5, 60), authProxy)
	authGroup.POST("/register", limiter.PerIP("register", 3, 60), authProxy)
	authGroup.GET("/me", limiter.PerUser("me", 60, 60), authProxy)
	authGroup.GET("/swagger/*any", authProxy)

	// /api/todos — explicit endpoints with tiered limits. CSV import/export
	// must come before the catch-all parameterised routes to win.
	todoGroup := api.Group("/todos")
	todoGroup.GET("/export", limiter.PerUser("todos-export", 5, 3600), todoProxy)
	todoGroup.POST("/import", limiter.PerUser("todos-import", 5, 3600), todoProxy)
	todoGroup.GET("", limiter.PerUser("todos-read", 120, 60), todoProxy)
	todoGroup.POST("", limiter.PerUser("todos-write", 60, 60), todoProxy)
	todoGroup.PUT("/:id", limiter.PerUser("todos-write", 60, 60), todoProxy)
	todoGroup.DELETE("/:id", limiter.PerUser("todos-write", 60, 60), todoProxy)

	addr := "0.0.0.0:" + port
	log.Printf("Gateway listening on %s; auth=%s, todo=%s", addr, authURL, todoURL)
	if err := router.Run(addr); err != nil {
		log.Fatal(err)
	}
}

func newRedis() *redis.Client {
	url := strings.TrimSpace(os.Getenv("REDIS_URL"))
	if url == "" {
		log.Println("REDIS_URL not set; rate limiting will fail-open")
		return nil
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		log.Printf("invalid REDIS_URL %q: %v; rate limiting will fail-open", url, err)
		return nil
	}
	c := redis.NewClient(opt)
	if err := c.Ping(context.Background()).Err(); err != nil {
		log.Printf("redis unreachable at %s: %v; rate limiting will fail-open", url, err)
		return nil
	}
	log.Printf("Redis connected at %s", url)
	return c
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
