package handlers

import (
	"auth-service/middleware"
	"auth-service/models"
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func Register(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body AuthRequest
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "email and password (min 6) required"})
			return
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
			return
		}

		user := models.User{
			ID:           uuid.New().String(),
			Email:        body.Email,
			PasswordHash: string(hash),
			CreatedAt:    time.Now(),
		}

		_, err = db.Exec(
			`INSERT INTO users (id, email, password_hash, created_at) VALUES ($1,$2,$3,$4)`,
			user.ID, user.Email, user.PasswordHash, user.CreatedAt,
		)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "email already taken"})
			return
		}

		token, err := middleware.GenerateToken(user.ID, user.Email)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create token"})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"user": user, "token": token})
	}
}

func Login(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body AuthRequest
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "email and password required"})
			return
		}

		var user models.User
		err := db.QueryRow(
			`SELECT id, email, password_hash, created_at FROM users WHERE email = $1`,
			body.Email,
		).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(body.Password)); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}

		token, err := middleware.GenerateToken(user.ID, user.Email)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create token"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"user": user, "token": token})
	}
}

func Me(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		uid, ok := userID.(string)
		if !ok || uid == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		var user models.User
		err := db.QueryRow(
			`SELECT id, email, password_hash, created_at FROM users WHERE id = $1`,
			uid,
		).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"user": user})
	}
}
