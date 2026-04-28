package main

import (
	"auth-service/database"
	_ "auth-service/docs"
	"auth-service/routes"
	"log"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

// @title           Auth Service API
// @version         1.0
// @description     User registration, login, and identity service. Issues JWTs consumed by todo-service.
// @host            localhost:8000
// @BasePath        /
// @schemes         http

// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
// @description                 Type "Bearer {token}" — get a token from /login or /register.
func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	db := database.InitDB()
	defer db.Close()

	routes.SetupRouter(db)
}
