// Package middleware provides HTTP middleware for the Fiber framework.
package middleware

import (
	"context"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"

	"github.com/yourusername/ecom-backend/pkg/response"
)

// Claims represents the verified JWT payload.
type Claims struct {
	UserID    string `json:"sub"`
	Role      string `json:"role"`
	SessionID string `json:"session_id"`
	jwt.RegisteredClaims
}

// AuthContextKey is the key used to store auth claims in Fiber context.
const AuthContextKey = "auth_claims"

const blacklistKeyPrefix = "blacklist:jti:"

// Auth is the JWT authentication middleware.
// It performs:
//  1. Authorization header parsing
//  2. HS256 signature verification
//  3. Expiry, issuer, audience validation
//  4. Redis blacklist check (post-logout token invalidation)
//  5. Claims extraction into Fiber locals
func Auth(accessSecret string, redisClient *redis.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return response.Unauthorized(c, "Authorization header is required")
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			return response.Unauthorized(c, "Invalid authorization format. Expected: Bearer <token>")
		}

		tokenString := strings.TrimSpace(parts[1])
		if tokenString == "" {
			return response.Unauthorized(c, "Token must not be empty")
		}

		// Parse and verify the JWT: signature, expiry, issuer, audience, algorithm.
		token, err := jwt.ParseWithClaims(
			tokenString,
			&Claims{},
			func(t *jwt.Token) (interface{}, error) {
				// Algorithm pinning: reject anything that isn't HS256.
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fiber.ErrUnauthorized
				}
				return []byte(accessSecret), nil
			},
			jwt.WithValidMethods([]string{"HS256"}),
			jwt.WithIssuer("ecom-backend"),
			jwt.WithAudience("ecom-api"),
			jwt.WithExpirationRequired(),
		)
		if err != nil || !token.Valid {
			return response.Unauthorized(c, "Invalid or expired access token")
		}

		claims, ok := token.Claims.(*Claims)
		if !ok || claims.UserID == "" {
			return response.Unauthorized(c, "Malformed token claims")
		}

		// Redis blacklist check — prevents use of tokens issued before logout.
		if redisClient != nil && claims.ID != "" {
			ctx := context.Background()
			exists, redisErr := redisClient.Exists(ctx, blacklistKeyPrefix+claims.ID).Result()
			if redisErr == nil && exists > 0 {
				return response.Unauthorized(c, "Token has been revoked. Please log in again")
			}
		}

		// Store verified claims in context for downstream handlers.
		c.Locals(AuthContextKey, claims)
		c.Locals("user_id", claims.UserID)
		c.Locals("role", claims.Role)
		c.Locals("session_id", claims.SessionID)
		c.Locals("token_jti", claims.ID)

		// Store remaining TTL for blacklisting on logout.
		if claims.ExpiresAt != nil {
			remaining := time.Until(claims.ExpiresAt.Time)
			if remaining > 0 {
				c.Locals("token_ttl", remaining)
			}
		}

		return c.Next()
	}
}

// JWTAuth is an alias for Auth (kept for backward compatibility).
func JWTAuth(accessSecret string, redisClient *redis.Client) fiber.Handler {
	return Auth(accessSecret, redisClient)
}

// RequireRole returns a middleware that allows only users with the specified roles.
// Must be used AFTER the Auth middleware.
//
// Example:
//
//	admin := v1.Group("/admin", authMw, middleware.RequireRole("ADMIN_OPS", "SUPER_ADMIN"))
func RequireRole(allowedRoles ...string) fiber.Handler {
	roleSet := make(map[string]struct{}, len(allowedRoles))
	for _, r := range allowedRoles {
		roleSet[r] = struct{}{}
	}
	return func(c *fiber.Ctx) error {
		role, _ := c.Locals("role").(string)
		if _, ok := roleSet[role]; !ok {
			return response.Forbidden(c, "You do not have permission to access this resource")
		}
		return c.Next()
	}
}
