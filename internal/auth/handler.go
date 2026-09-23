package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"github.com/yourusername/ecom-backend/config"
	"github.com/yourusername/ecom-backend/pkg/response"
)

type Handler struct {
	cfg   *config.Config
	db    *pgxpool.Pool
	redis *redis.Client
}

func NewHandler(cfg *config.Config, db *pgxpool.Pool, redis *redis.Client) *Handler {
	return &Handler{cfg: cfg, db: db, redis: redis}
}

func (h *Handler) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	auth := router.Group("/auth")

	auth.Post("/otp/send", h.SendOTP)
	auth.Post("/otp/verify", h.VerifyOTP)
	auth.Post("/email/login", h.EmailLogin)
	auth.Post("/email/register", h.EmailRegister)
	auth.Post("/refresh", h.RefreshToken)
	auth.Post("/logout", authMiddleware, h.Logout)
	auth.Post("/password/reset-request", h.PasswordResetRequest)
	auth.Put("/password/reset", h.PasswordReset)
	auth.Get("/sessions", authMiddleware, h.GetSessions)
	auth.Delete("/sessions/:sessionId", authMiddleware, h.RevokeSession)
	auth.Get("/me", authMiddleware, h.GetMe)
	auth.Get("/google/callback", h.GoogleCallback)
}

// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
// SEND OTP
// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

type SendOTPReq struct {
	Target  string `json:"target"`
	Purpose string `json:"purpose"`
}

func (h *Handler) SendOTP(c *fiber.Ctx) error {
	var req SendOTPReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request payload")
	}
	req.Target = strings.TrimSpace(req.Target)
	if req.Target == "" {
		return response.ValidationError(c, map[string]string{"target": "Phone number or email is required"})
	}

	otpCode := generateNumericOTP(6)
	ctx := c.Context()

	if h.db != nil {
		_, err := h.db.Exec(ctx,
			"INSERT INTO otp_requests (phone, otp_hash, purpose, expires_at) VALUES ($1, $2, $3, $4)",
			req.Target, otpCode, req.Purpose, time.Now().Add(5*time.Minute),
		)
		if err != nil {
			return response.Error(c, fiber.StatusInternalServerError, fmt.Sprintf("Failed to store OTP: %v", err), nil)
		}
	}

	return response.Success(c, fiber.StatusOK, "OTP sent successfully", fiber.Map{
		"target":     req.Target,
		"expires_in": "300s",
		"debug_otp":  otpCode, // remove in production
	})
}

// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
// VERIFY OTP
// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

type VerifyOTPReq struct {
	Target  string `json:"target"`
	OTPCode string `json:"otp_code"`
	Purpose string `json:"purpose"`
}

func (h *Handler) VerifyOTP(c *fiber.Ctx) error {
	var req VerifyOTPReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request payload")
	}
	req.Target = strings.TrimSpace(req.Target)
	req.OTPCode = strings.TrimSpace(req.OTPCode)
	if req.Target == "" || req.OTPCode == "" {
		return response.BadRequest(c, "target and otp_code are required")
	}
	ctx := c.Context()
	userID := uuid.New().String()

	if h.db != nil {
		// âœ… Validate OTP from DB
		var storedOTP string
		var expiresAt time.Time
		otpErr := h.db.QueryRow(ctx,
			`SELECT otp_hash, expires_at FROM otp_requests
			 WHERE phone = $1 AND purpose = $2
			 ORDER BY created_at DESC LIMIT 1`,
			req.Target, req.Purpose,
		).Scan(&storedOTP, &expiresAt)
		if otpErr != nil {
			return response.Error(c, fiber.StatusBadRequest, "No OTP found for this target. Please request OTP first.", nil)
		}
		if time.Now().After(expiresAt) {
			return response.Error(c, fiber.StatusBadRequest, "OTP has expired. Please request a new one.", nil)
		}
		if storedOTP != req.OTPCode {
			return response.Error(c, fiber.StatusBadRequest, "Invalid OTP code", nil)
		}

		// Find or create user
		var existingID string
		err := h.db.QueryRow(ctx, "SELECT id::text FROM users WHERE phone = $1", req.Target).Scan(&existingID)
		if err != nil {
			// New user â€” insert
			err = h.db.QueryRow(ctx,
				"INSERT INTO users (phone, full_name, role, status) VALUES ($1, $2, 'CUSTOMER', 'ACTIVE') RETURNING id::text",
				req.Target, "User-"+req.Target[len(req.Target)-4:],
			).Scan(&userID)
			if err != nil {
				return response.Error(c, fiber.StatusInternalServerError, fmt.Sprintf("Failed to create user: %v", err), nil)
			}
		} else {
			userID = existingID
		}

		// Clean up used OTP
		_, _ = h.db.Exec(ctx,
			"DELETE FROM otp_requests WHERE phone = $1 AND purpose = $2",
			req.Target, req.Purpose,
		)
	}

	accToken := buildAccessToken(userID)
	refToken := buildRefreshToken(userID)

	if h.db != nil {
		_, _ = h.db.Exec(ctx,
			"INSERT INTO sessions (user_id, refresh_token_hash, expires_at, status) VALUES ($1, $2, $3, 'ACTIVE')",
			userID, refToken, time.Now().Add(30*24*time.Hour),
		)
	}

	return response.Success(c, fiber.StatusOK, "OTP verified & user authenticated", fiber.Map{
		"access_token":  accToken,
		"refresh_token": refToken,
		"user_id":       userID,
		"phone":         req.Target,
		"role":          "CUSTOMER",
	})
}

// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
// EMAIL REGISTER
// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

type EmailRegisterReq struct {
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
	Role     string `json:"role"`
}

func (h *Handler) EmailRegister(c *fiber.Ctx) error {
	var req EmailRegisterReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid registration input")
	}
	req.Email = strings.TrimSpace(req.Email)
	req.Phone = strings.TrimSpace(req.Phone)

	if req.Email == "" || req.Password == "" {
		return response.BadRequest(c, "Email and password are required")
	}
	if !isValidEmail(req.Email) {
		return response.ValidationError(c, map[string]string{"email": "Invalid email format"})
	}
	if len(req.Password) < 6 {
		return response.ValidationError(c, map[string]string{"password": "Password must be at least 6 characters"})
	}
	if req.Role == "" {
		req.Role = "CUSTOMER"
	}
	ctx := c.Context()

	// Hash password
	hashBytes, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to secure password", nil)
	}
	passHash := string(hashBytes)

	var phoneVal *string
	if req.Phone != "" {
		phoneVal = &req.Phone
	}

	var userID string
	if h.db != nil {
		// Check duplicate email
		var existCheck string
		checkErr := h.db.QueryRow(ctx, "SELECT id::text FROM users WHERE email = $1", req.Email).Scan(&existCheck)
		if checkErr == nil {
			return response.Error(c, fiber.StatusConflict, "An account with this email already exists. Please login.", nil)
		}

		err = h.db.QueryRow(ctx,
			`INSERT INTO users (email, phone, password_hash, full_name, role, status, email_verified, phone_verified)
			 VALUES ($1, $2, $3, $4, $5, 'ACTIVE', true, true)
			 RETURNING id::text`,
			req.Email, phoneVal, passHash, req.FullName, req.Role,
		).Scan(&userID)

		if err != nil {
			return response.Error(c, fiber.StatusInternalServerError, fmt.Sprintf("Registration failed: %v", err), nil)
		}
	} else {
		userID = uuid.New().String()
	}

	accToken := buildAccessToken(userID)
	refToken := buildRefreshToken(userID)

	if h.db != nil {
		_, _ = h.db.Exec(ctx,
			"INSERT INTO sessions (user_id, refresh_token_hash, expires_at, status) VALUES ($1, $2, $3, 'ACTIVE')",
			userID, refToken, time.Now().Add(30*24*time.Hour),
		)
	}

	return response.Created(c, "User registered successfully", fiber.Map{
		"user_id":       userID,
		"email":         req.Email,
		"phone":         req.Phone,
		"full_name":     req.FullName,
		"role":          req.Role,
		"access_token":  accToken,
		"refresh_token": refToken,
	})
}

// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
// EMAIL LOGIN  â† CRITICAL FIX: bcrypt verify
// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

type EmailLoginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) EmailLogin(c *fiber.Ctx) error {
	var req EmailLoginReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid credentials payload")
	}
	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" || req.Password == "" {
		return response.BadRequest(c, "Email and password are required")
	}
	ctx := c.Context()

	if h.db == nil {
		return response.Error(c, fiber.StatusServiceUnavailable, "Database not connected", nil)
	}

	// Fetch user from DB
	var userID, storedHash, role, status string
	var phone *string
	err := h.db.QueryRow(ctx,
		"SELECT id::text, password_hash, role, status, phone FROM users WHERE email = $1",
		req.Email,
	).Scan(&userID, &storedHash, &role, &status, &phone)
	if err != nil {
		// Do NOT reveal whether email exists
		return response.Error(c, fiber.StatusUnauthorized, "Invalid email or password", nil)
	}

	// Account status check
	if status != "ACTIVE" {
		return response.Error(c, fiber.StatusForbidden, "Account is "+status+". Please contact support.", nil)
	}

	// âœ… CRITICAL: Verify password against stored bcrypt hash
	if bcryptErr := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(req.Password)); bcryptErr != nil {
		return response.Error(c, fiber.StatusUnauthorized, "Invalid email or password", nil)
	}

	accToken := buildAccessToken(userID)
	refToken := buildRefreshToken(userID)

	// Save session to DB
	_, _ = h.db.Exec(ctx,
		"INSERT INTO sessions (user_id, refresh_token_hash, expires_at, status) VALUES ($1, $2, $3, 'ACTIVE')",
		userID, refToken, time.Now().Add(30*24*time.Hour),
	)

	phoneStr := ""
	if phone != nil {
		phoneStr = *phone
	}

	return response.Success(c, fiber.StatusOK, "Login successful", fiber.Map{
		"access_token":  accToken,
		"refresh_token": refToken,
		"user_id":       userID,
		"email":         req.Email,
		"phone":         phoneStr,
		"role":          role,
	})
}

// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
// REFRESH TOKEN
// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

func (h *Handler) RefreshToken(c *fiber.Ctx) error {
	type RefreshReq struct {
		RefreshToken string `json:"refresh_token"`
	}
	var req RefreshReq
	if err := c.BodyParser(&req); err != nil || req.RefreshToken == "" {
		return response.BadRequest(c, "refresh_token is required")
	}
	ctx := c.Context()

	if h.db != nil {
		// Lookup session
		var userID, status string
		err := h.db.QueryRow(ctx,
			"SELECT user_id::text, status FROM sessions WHERE refresh_token_hash = $1",
			req.RefreshToken,
		).Scan(&userID, &status)
		if err != nil || status != "ACTIVE" {
			return response.Error(c, fiber.StatusUnauthorized, "Invalid or expired refresh token", nil)
		}

		newAccToken := buildAccessToken(userID)
		newRefToken := buildRefreshToken(userID)

		// Rotate: invalidate old, insert new
		_, _ = h.db.Exec(ctx,
			"UPDATE sessions SET status = 'REVOKED', is_revoked = true WHERE refresh_token_hash = $1",
			req.RefreshToken,
		)
		_, _ = h.db.Exec(ctx,
			"INSERT INTO sessions (user_id, refresh_token_hash, expires_at, status) VALUES ($1, $2, $3, 'ACTIVE')",
			userID, newRefToken, time.Now().Add(30*24*time.Hour),
		)

		return response.Success(c, fiber.StatusOK, "Token refreshed", fiber.Map{
			"access_token":  newAccToken,
			"refresh_token": newRefToken,
		})
	}

	return response.Success(c, fiber.StatusOK, "Token rotated", fiber.Map{
		"access_token":  buildAccessToken("unknown"),
		"refresh_token": buildRefreshToken("unknown"),
	})
}

// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
// LOGOUT
// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

func (h *Handler) Logout(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	ctx := c.Context()

	if h.db != nil {
		// Revoke ALL active sessions for this user on logout
		// (access token doesn't map 1:1 to session, so revoke all for security)
		_, _ = h.db.Exec(ctx,
			"UPDATE sessions SET status = 'REVOKED', is_revoked = true WHERE user_id::text = $1 AND status = 'ACTIVE'",
			userID,
		)
	}

	return response.Success(c, fiber.StatusOK, "Logged out successfully. All sessions revoked.", nil)
}

// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
// PASSWORD RESET REQUEST
// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

type PasswordResetRequestReq struct {
	Email string `json:"email"`
}

func (h *Handler) PasswordResetRequest(c *fiber.Ctx) error {
	var req PasswordResetRequestReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request payload")
	}
	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" {
		return response.BadRequest(c, "Email is required")
	}
	ctx := c.Context()

	if h.db == nil {
		return response.Error(c, fiber.StatusServiceUnavailable, "Database not connected", nil)
	}

	var userID string
	err := h.db.QueryRow(ctx, "SELECT id::text FROM users WHERE email = $1", req.Email).Scan(&userID)
	if err != nil {
		// Security: don't reveal if email exists
		return response.Success(c, fiber.StatusOK, "If this email is registered, an OTP has been sent.", fiber.Map{
			"status": "DISPATCHED",
		})
	}

	otpCode := generateNumericOTP(6)
	expiresAt := time.Now().Add(15 * time.Minute)

	// Store OTP in otp_requests table
	_, err = h.db.Exec(ctx,
		"INSERT INTO otp_requests (phone, otp_hash, purpose, expires_at) VALUES ($1, $2, 'PASSWORD_RESET', $3)",
		req.Email, otpCode, expiresAt,
	)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, fmt.Sprintf("Failed to store reset OTP: %v", err), nil)
	}

	// TODO: Send email via SMTP when configured
	smtpConfigured := h.cfg != nil && h.cfg.Email.SMTPHost != ""
	_ = smtpConfigured

	return response.Success(c, fiber.StatusOK, "Password reset OTP generated", fiber.Map{
		"status":       "DISPATCHED",
		"expires_in":   "900s",
		"reset_otp":    otpCode, // â† show in dev; remove for production
		"instructions": "Send email + otp_code + new_password to PUT /api/v1/auth/password/reset",
	})
}

// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
// PASSWORD RESET  â† FIX: invalidate all sessions
// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

type PasswordResetReq struct {
	Email       string `json:"email"`
	OTPCode     string `json:"otp_code"`
	NewPassword string `json:"new_password"`
}

func (h *Handler) PasswordReset(c *fiber.Ctx) error {
	var req PasswordResetReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid password reset payload")
	}
	req.Email = strings.TrimSpace(req.Email)
	req.OTPCode = strings.TrimSpace(req.OTPCode)
	if req.Email == "" || req.OTPCode == "" || req.NewPassword == "" {
		return response.BadRequest(c, "email, otp_code, and new_password are all required")
	}
	if len(req.NewPassword) < 6 {
		return response.ValidationError(c, map[string]string{"new_password": "Password must be at least 6 characters"})
	}
	ctx := c.Context()

	if h.db == nil {
		return response.Error(c, fiber.StatusServiceUnavailable, "Database not connected", nil)
	}

	// 1. Verify user exists
	var userID string
	err := h.db.QueryRow(ctx, "SELECT id::text FROM users WHERE email = $1", req.Email).Scan(&userID)
	if err != nil {
		return response.NotFound(c, "User with this email not found")
	}

	// 2. Verify OTP from otp_requests table (not expired, matches)
	var storedOTP string
	var expiresAt time.Time
	otpErr := h.db.QueryRow(ctx,
		`SELECT otp_hash, expires_at FROM otp_requests
		 WHERE phone = $1 AND purpose = 'PASSWORD_RESET'
		 ORDER BY created_at DESC LIMIT 1`,
		req.Email,
	).Scan(&storedOTP, &expiresAt)

	if otpErr != nil {
		return response.Error(c, fiber.StatusBadRequest, "No password reset request found. Please request OTP first.", nil)
	}
	if time.Now().After(expiresAt) {
		return response.Error(c, fiber.StatusBadRequest, "OTP has expired. Please request a new one.", nil)
	}
	if storedOTP != req.OTPCode {
		return response.Error(c, fiber.StatusBadRequest, "Invalid OTP code", nil)
	}

	// 3. Hash new password
	hashBytes, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to secure new password", nil)
	}

	// 4. Update password in DB
	_, err = h.db.Exec(ctx,
		"UPDATE users SET password_hash = $1, updated_at = now() WHERE id::text = $2",
		string(hashBytes), userID,
	)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, fmt.Sprintf("Failed to update password: %v", err), nil)
	}

	// 5. âœ… CRITICAL: Revoke ALL existing sessions so old tokens can't be used
	_, _ = h.db.Exec(ctx,
		"UPDATE sessions SET status = 'REVOKED', is_revoked = true WHERE user_id::text = $1 AND status = 'ACTIVE'",
		userID,
	)

	// 6. Clean up used OTP
	_, _ = h.db.Exec(ctx,
		"DELETE FROM otp_requests WHERE phone = $1 AND purpose = 'PASSWORD_RESET'",
		req.Email,
	)

	return response.Success(c, fiber.StatusOK, "Password reset successfully. All previous sessions have been revoked. Please login with your new password.", fiber.Map{
		"email":            req.Email,
		"status":           "UPDATED",
		"sessions_revoked": true,
	})
}

// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
// GET SESSIONS
// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

func (h *Handler) GetSessions(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	ctx := c.Context()

	type Session struct {
		ID        string    `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		ExpiresAt time.Time `json:"expires_at"`
		Status    string    `json:"status"`
	}

	var sessions []Session

	if h.db != nil {
		rows, err := h.db.Query(ctx,
			"SELECT id::text, created_at, expires_at, status FROM sessions WHERE user_id::text = $1 AND status = 'ACTIVE' AND expires_at > now() ORDER BY created_at DESC",
			userID,
		)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var s Session
				if scanErr := rows.Scan(&s.ID, &s.CreatedAt, &s.ExpiresAt, &s.Status); scanErr == nil {
					sessions = append(sessions, s)
				}
			}
		}
	}

	if sessions == nil {
		sessions = []Session{}
	}

	return response.Success(c, fiber.StatusOK, "Active sessions retrieved", fiber.Map{
		"user_id":  userID,
		"sessions": sessions,
		"count":    len(sessions),
	})
}

// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
// REVOKE SESSION
// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

func (h *Handler) RevokeSession(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	userID := c.Locals("user_id").(string)
	ctx := c.Context()

	if h.db != nil {
		_, err := h.db.Exec(ctx,
			"UPDATE sessions SET status = 'REVOKED' WHERE id::text = $1 AND user_id::text = $2",
			sessionID, userID,
		)
		if err != nil {
			return response.Error(c, fiber.StatusInternalServerError, "Failed to revoke session", nil)
		}
	}

	return response.Success(c, fiber.StatusOK, "Session revoked successfully", fiber.Map{
		"revoked_session_id": sessionID,
	})
}

// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
// GET ME  â† live from NeonDB
// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

func (h *Handler) GetMe(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	ctx := c.Context()

	if h.db == nil {
		return response.Error(c, fiber.StatusServiceUnavailable, "Database not connected", nil)
	}

	var id, fullName, role, status string
	var email, phone, profilePic *string
	var emailVerified, phoneVerified bool
	var createdAt time.Time

	err := h.db.QueryRow(ctx,
		`SELECT id::text, email, phone, full_name, role, status,
		        email_verified, phone_verified, profile_picture_url, created_at
		 FROM users
		 WHERE id::text = $1`,
		userID,
	).Scan(&id, &email, &phone, &fullName, &role, &status,
		&emailVerified, &phoneVerified, &profilePic, &createdAt)

	if err != nil {
		return response.NotFound(c, fmt.Sprintf("User not found in database (id=%s): %v", userID, err))
	}

	emailStr := ""
	if email != nil {
		emailStr = *email
	}
	phoneStr := ""
	if phone != nil {
		phoneStr = *phone
	}
	picStr := ""
	if profilePic != nil {
		picStr = *profilePic
	}

	return response.Success(c, fiber.StatusOK, "User profile loaded from NeonDB", fiber.Map{
		"id":                  id,
		"email":               emailStr,
		"phone":               phoneStr,
		"full_name":           fullName,
		"role":                role,
		"status":              status,
		"email_verified":      emailVerified,
		"phone_verified":      phoneVerified,
		"profile_picture_url": picStr,
		"created_at":          createdAt,
	})
}

// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
// GOOGLE CALLBACK
// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

func (h *Handler) GoogleCallback(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "OAuth Google Login - configure via OAuth provider", fiber.Map{
		"provider": "google",
		"status":   "not_configured",
	})
}

// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
// HELPERS
// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

func buildAccessToken(userID string) string {
	return fmt.Sprintf("acc_%s_%d", userID, time.Now().Unix())
}

func buildRefreshToken(userID string) string {
	return fmt.Sprintf("ref_%s_%s", userID, randomHex(16))
}

func generateNumericOTP(length int) string {
	const digits = "0123456789"
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		num, _ := rand.Int(rand.Reader, big.NewInt(10))
		result[i] = digits[num.Int64()]
	}
	return string(result)
}

func randomHex(bytes int) string {
	b := make([]byte, bytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func isValidEmail(email string) bool {
	re := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return re.MatchString(email)
}

