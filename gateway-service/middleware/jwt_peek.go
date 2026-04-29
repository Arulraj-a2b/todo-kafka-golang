package middleware

import (
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// peekUserID decodes a JWT WITHOUT verifying the signature and returns the
// `user_id` claim. Used purely for rate-limit keying — downstream services
// still validate the token. Returns "" if the header is missing or malformed.
func peekUserID(authHeader string) string {
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return ""
	}
	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	tok, _, err := parser.ParseUnverified(tokenStr, jwt.MapClaims{})
	if err != nil {
		return ""
	}
	claims, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		return ""
	}
	if uid, ok := claims["user_id"].(string); ok {
		return uid
	}
	return ""
}
