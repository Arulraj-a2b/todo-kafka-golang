package main

import (
	"database/sql"
	"log"
	"todo-kafka/database"
	_ "todo-kafka/docs"
	"todo-kafka/kafka/consumer"
	"todo-kafka/kafka/producer"
	"todo-kafka/routes"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

var db *sql.DB

// @title           Todo-Kafka API
// @version         1.0
// @description     Kafka-backed todo service with JWT auth and CSV import/export.
// @host            localhost:8000
// @BasePath        /
// @schemes         http

// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
// @description                 Type "Bearer {token}" — get a token from /login or /register.
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
