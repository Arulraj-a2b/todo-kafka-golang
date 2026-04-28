package main

import (
	"database/sql"
	"log"
	"todo-service/database"
	_ "todo-service/docs"
	"todo-service/kafka/consumer"
	"todo-service/kafka/producer"
	"todo-service/routes"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

var db *sql.DB

// @title           Todo Service API
// @version         1.0
// @description     Kafka-backed todo service. Verifies JWTs issued by auth-service.
// @host            localhost:8001
// @BasePath        /
// @schemes         http

// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
// @description                 Type "Bearer {token}" — obtain one from auth-service at :8000.
func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found")
	}

	db := database.InitDB()
	defer db.Close()

	producer.InitKafka()
	defer producer.KafkaWriter.Close()

	go consumer.InitConsumer(db)

	routes.SetupRouter(db)
}
