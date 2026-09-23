package middleware

import (
	"context"
	"fmt"
	"strings"
	"time"

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
// It validates the Bearer token in the Authorization header.
func Auth(accessSecret string, redisClient *redis.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Extract token from Authorization header
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return response.Unauthorized(c, "Authorization header required")
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			return response.Unauthorized(c, "Invalid authorization format. Use: Bearer <token>")
		}

		tokenString := parts[1]

		// Parse and validate JWT
		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return []byte(accessSecret), nil
		})

		if err != nil || !token.Valid {
			return response.Unauthorized(c, "Invalid or expired token")
		}

		// Check if token is blacklisted (logout)
		blacklistKey := fmt.Sprintf("blacklist:token:%s", tokenString[:min(32, len(tokenString))])
		exists, err := redisClient.Exists(context.Background(), blacklistKey).Result()
		if err == nil && exists > 0 {
			return response.Unauthorized(c, "Token has been revoked")
		}

		// Store claims in context for handlers to use
		c.Locals(AuthContextKey, claims)
		c.Locals("user_id", claims.UserID)
		c.Locals("user_role", claims.Role)

		return c.Next()
	}
}

// RequireRole checks if the authenticated user has one of the allowed roles.
// Must be used after Auth middleware.
func RequireRole(roles ...string) fiber.Handler {
	roleSet := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		roleSet[r] = struct{}{}
	}

	return func(c *fiber.Ctx) error {
		claims, ok := c.Locals(AuthContextKey).(*Claims)
		if !ok {
			return response.Unauthorized(c, "Authentication required")
		}

		if _, allowed := roleSet[claims.Role]; !allowed {
			return response.Forbidden(c, "You don't have permission for this action")
		}

		return c.Next()
	}
}

// OptionalAuth attempts to authenticate but doesn't block if no token is present.
// Useful for endpoints that serve both public and authenticated users differently.
func OptionalAuth(accessSecret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Next()
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			return c.Next()
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(parts[1], claims, func(t *jwt.Token) (interface{}, error) {
			return []byte(accessSecret), nil
		})

		if err == nil && token.Valid {
			c.Locals(AuthContextKey, claims)
			c.Locals("user_id", claims.UserID)
			c.Locals("user_role", claims.Role)
		}

		return c.Next()
	}
}

// GetAuthClaims extracts auth claims from Fiber context.
// Returns nil if user is not authenticated.
func GetAuthClaims(c *fiber.Ctx) *Claims {
	claims, _ := c.Locals(AuthContextKey).(*Claims)
	return claims
}

// GetUserID extracts the authenticated user's ID from context.
func GetUserID(c *fiber.Ctx) string {
	return c.Locals("user_id").(string)
}

// GenerateAccessToken creates a new JWT access token.
func GenerateAccessToken(userID, role, sessionID, secret string, expiry time.Duration) (string, error) {
	claims := Claims{
		UserID:    userID,
		Role:      role,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// GenerateRefreshToken creates a new JWT refresh token.
func GenerateRefreshToken(userID, sessionID, secret string, expiry time.Duration) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		ID:        sessionID,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
