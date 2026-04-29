package handlers

import (
	"auth-service/cache"
	"auth-service/middleware"
	"auth-service/models"
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	userCacheTTL  = 5 * time.Minute
	loginCacheTTL = 1 * time.Minute
)

func userCacheKey(userID string) string  { return "user:" + userID }
func loginCacheKey(email string) string  { return "auth:user_by_email:" + email }

// loginCacheRecord is the minimal record for cache-aside on login lookup —
// just what bcrypt.CompareHashAndPassword needs.
type loginCacheRecord struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"password_hash"`
	CreatedAt    time.Time `json:"created_at"`
}

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
// @Description  Authenticates an existing user and returns a JWT. User lookup is cached in Redis (1 min TTL) to absorb retry bursts.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      AuthRequest  true  "Credentials"
// @Success      200   {object}  AuthResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /login [post]
func Login(db *sql.DB, cc *cache.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body AuthRequest
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "email and password required"})
			return
		}

		rec, err := loadUserByEmailCached(c.Request.Context(), db, cc, body.Email)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(rec.PasswordHash), []byte(body.Password)); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}

		token, err := middleware.GenerateToken(rec.ID, rec.Email)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create token"})
			return
		}

		user := models.User{
			ID:           rec.ID,
			Email:        rec.Email,
			PasswordHash: rec.PasswordHash,
			CreatedAt:    rec.CreatedAt,
		}
		c.JSON(http.StatusOK, gin.H{"user": user, "token": token})
	}
}

// loadUserByEmailCached implements cache-aside on the login lookup. Short
// TTL (1 min) absorbs password-retry bursts without holding stale credentials.
func loadUserByEmailCached(ctx context.Context, db *sql.DB, cc *cache.Client, email string) (loginCacheRecord, error) {
	key := loginCacheKey(email)
	if cached, err := cc.Get(ctx, key); err == nil {
		var rec loginCacheRecord
		if jerr := json.Unmarshal(cached, &rec); jerr == nil {
			return rec, nil
		}
	} else if err != nil && !cache.IsMiss(err) {
		log.Printf("cache get %s: %v", key, err)
	}

	var rec loginCacheRecord
	err := db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, created_at FROM users WHERE email = $1`,
		email,
	).Scan(&rec.ID, &rec.Email, &rec.PasswordHash, &rec.CreatedAt)
	if err != nil {
		return loginCacheRecord{}, err
	}

	if body, jerr := json.Marshal(rec); jerr == nil {
		cc.Set(ctx, key, body, loginCacheTTL)
	}
	return rec, nil
}

// Me godoc
// @Summary      Get the current user
// @Description  Returns the authenticated user identified by the JWT. User record is cached in Redis (5 min TTL).
// @Tags         auth
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  MeResponse
// @Failure      401  {object}  ErrorResponse
// @Router       /me [get]
func Me(db *sql.DB, cc *cache.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		uid, ok := userID.(string)
		if !ok || uid == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		key := userCacheKey(uid)
		if cached, err := cc.Get(c.Request.Context(), key); err == nil {
			c.Data(http.StatusOK, "application/json", cached)
			return
		} else if err != nil && !cache.IsMiss(err) {
			log.Printf("cache get %s: %v", key, err)
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

		body, err := json.Marshal(gin.H{"user": user})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "encode failed"})
			return
		}
		cc.Set(c.Request.Context(), key, body, userCacheTTL)
		c.Data(http.StatusOK, "application/json", body)
	}
}
