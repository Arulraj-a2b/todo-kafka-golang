package main

import (
	"context"
	"log/slog"
	"time"

	"auth-service/cache"
	"auth-service/database"
	"auth-service/internal/obs"
	"auth-service/routes"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

// @title           Auth Service API
// @version         1.0
// @description     User registration, login, and identity service. Issues JWTs consumed by todo-service. User lookups are cached in Redis.
// @host            localhost:8000
// @BasePath        /
// @schemes         http

// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
// @description                 Type "Bearer {token}" — get a token from /login or /register.
func main() {
	if err := godotenv.Load(); err != nil {
		// Non-fatal — production reads env directly from the orchestrator.
		// Logging via stdlib log here because slog isn't initialized yet.
	}

	obs.InitLogger("auth-service")
	obs.InitMetrics()

	ctx := context.Background()
	shutdown := obs.InitTracer(ctx, "auth-service")
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdown(ctx); err != nil {
			slog.Error("tracer shutdown", "err", err)
		}
	}()

	db := database.InitDB()
	defer db.Close()

	cc := cache.New()

	routes.SetupRouter(db, cc)
}
