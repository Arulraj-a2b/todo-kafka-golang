package main

import (
	"context"
	"log/slog"
	"time"

	"todo-service/cache"
	"todo-service/database"
	"todo-service/internal/obs"
	"todo-service/kafka/producer"
	"todo-service/routes"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

// @title           Todo Service API
// @version         1.0
// @description     Stateless todo service. Writes to Postgres synchronously and publishes events to Kafka for downstream consumers (notification-worker, cache-invalidator). Verifies JWTs issued by auth-service.
// @host            localhost:8001
// @BasePath        /
// @schemes         http

// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
// @description                 Type "Bearer {token}" — obtain one from auth-service at :8000.
func main() {
	_ = godotenv.Load()

	obs.InitLogger("todo-service")
	obs.InitMetrics()

	ctx := context.Background()
	shutdown := obs.InitTracer(ctx, "todo-service")
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdown(ctx); err != nil {
			slog.Error("tracer shutdown", "err", err)
		}
	}()

	db := database.InitDB()
	defer db.Close()

	producer.InitKafka()
	defer func() {
		if producer.KafkaWriter != nil {
			producer.KafkaWriter.Close()
		}
	}()

	cc := cache.New()

	routes.SetupRouter(db, cc)
}
