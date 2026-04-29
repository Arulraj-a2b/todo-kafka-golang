package main

import (
	"log"
	"todo-service/cache"
	"todo-service/database"
	_ "todo-service/docs"
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
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

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
