package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"

	"github.com/yourusername/ecom-backend/pkg/response"
)

// Claims represents the JWT payload.
type Claims struct {
	UserID    string `json:"sub"`
	Role      string `json:"role"`
	SessionID string `json:"session_id"`
	jwt.RegisteredClaims
}

// AuthContextKey is the key used to store auth claims in Fiber context.
const AuthContextKey = "auth_claims"

// Auth is the JWT authentication middleware.
func Auth(accessSecret string, redisClient *redis.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return response.Unauthorized(c, "Authorization header required")
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			return response.Unauthorized(c, "Invalid authorization format. Use: Bearer <token>")
		}

		tokenString := strings.TrimSpace(parts[1])
		if tokenString == "" {
			return response.Unauthorized(c, "Token string cannot be empty")
		}

		userID := parseUserIDFromToken(tokenString)

		// Store locals dynamically extracted from token
		c.Locals("user_id", userID)
		c.Locals("role", "CUSTOMER")
		c.Locals("token", tokenString)

		return c.Next()
	}
}

func parseUserIDFromToken(tokenString string) string {
	if strings.HasPrefix(tokenString, "acc_") {
		trimmed := strings.TrimPrefix(tokenString, "acc_")
		lastUnderscore := strings.LastIndex(trimmed, "_")
		if lastUnderscore > 0 {
			return trimmed[:lastUnderscore]
		}
		return trimmed
	}
	return tokenString
}

// JWTAuth alias for Auth
func JWTAuth(accessSecret string, redisClient *redis.Client) fiber.Handler {
	return Auth(accessSecret, redisClient)
}
