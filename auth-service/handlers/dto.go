package handlers

import "auth-service/models"

type AuthRequest struct {
	Email    string `json:"email" binding:"required,email" example:"user@example.com"`
	Password string `json:"password" binding:"required,min=6" example:"secret123"`
}

type AuthResponse struct {
	User  models.User `json:"user"`
	Token string      `json:"token"`
}

type MeResponse struct {
	User models.User `json:"user"`
}

type ErrorResponse struct {
	Error string `json:"error" example:"invalid request body"`
}
