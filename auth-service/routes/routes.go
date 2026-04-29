package routes

import (
	"auth-service/cache"
	"auth-service/handlers"
	"auth-service/middleware"
	"database/sql"
	"os"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func SetupRouter(db *sql.DB, cc *cache.Client) *gin.Engine {
	router := gin.Default()

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	router.POST("/register", handlers.Register(db))
	router.POST("/login", handlers.Login(db, cc))

	auth := router.Group("/", middleware.JWT())
	auth.GET("/me", handlers.Me(db, cc))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}
	router.Run("0.0.0.0:" + port)
	return router
}
