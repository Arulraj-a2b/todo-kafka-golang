package main

import (
	"auth-service/database"
	"auth-service/routes"
	"log"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	db := database.InitDB()
	defer db.Close()

	routes.SetupRouter(db)
}
