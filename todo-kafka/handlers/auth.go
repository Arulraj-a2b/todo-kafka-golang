package handlers

import (
	"database/sql"
	"net/http"
	"time"
	"todo-kafka/middleware"
	"todo-kafka/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Register godoc
// @Summary      Register a new user
// @Description  Creates a user and returns a JWT.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      AuthRequest  true  "Credentials"
// @Success      201   {object}  AuthResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /register [post]
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

// Login godoc
// @Summary      Log in
// @Description  Authenticates an existing user and returns a JWT.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      AuthRequest  true  "Credentials"
// @Success      200   {object}  AuthResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /login [post]
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

// Me godoc
// @Summary      Get the current user
// @Description  Returns the authenticated user identified by the JWT.
// @Tags         auth
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  MeResponse
// @Failure      401  {object}  ErrorResponse
// @Router       /me [get]
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
