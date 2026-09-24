package auth

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/yourusername/ecom-backend/config"
	"github.com/yourusername/ecom-backend/pkg/response"
)

type Handler struct {
	cfg     *config.Config
	db      *pgxpool.Pool
	redis   *redis.Client
	service *Service
}

func NewHandler(cfg *config.Config, db *pgxpool.Pool, redisClient *redis.Client) *Handler {
	repo := NewRepository(db)
	service := NewService(repo)
	return &Handler{
		cfg:     cfg,
		db:      db,
		redis:   redisClient,
		service: service,
	}
}

func (h *Handler) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	auth := router.Group("/auth")

	auth.Post("/otp/send", h.SendOTP)
	auth.Post("/phone/send-otp", h.SendOTP)
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

type SendOTPReq struct {
	Target  string `json:"target"`
	Phone   string `json:"phone"`
	Purpose string `json:"purpose"`
}

func (h *Handler) SendOTP(c *fiber.Ctx) error {
	var req SendOTPReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request payload")
	}
	if req.Target == "" {
		req.Target = req.Phone
	}
	req.Target = strings.TrimSpace(req.Target)
	if req.Purpose == "" {
		req.Purpose = "LOGIN"
	}
	if req.Target == "" {
		return response.ValidationError(c, map[string]string{"target": "Phone number or email is required"})
	}

	result, err := h.service.SendOTP(c.Context(), req.Target, req.Purpose)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, fmt.Sprintf("Failed to store OTP: %v", err), nil)
	}

	return response.Success(c, fiber.StatusOK, "OTP sent successfully", fiber.Map{
		"target":     req.Target,
		"expires_in": result.ExpiresIn,
		"debug_otp":  result.OTPCode,
	})
}

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

	result, err := h.service.VerifyOTP(c.Context(), req.Target, req.OTPCode, req.Purpose, h.db != nil)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error(), nil)
	}

	return response.Success(c, fiber.StatusOK, "OTP verified & user authenticated", fiber.Map{
		"access_token":  result.AccessToken,
		"refresh_token": result.RefToken,
		"user_id":       result.UserID,
		"phone":         result.Phone,
		"role":          "CUSTOMER",
	})
}

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
	if !h.service.IsValidEmail(req.Email) {
		return response.ValidationError(c, map[string]string{"email": "Invalid email format"})
	}
	if len(req.Password) < 6 {
		return response.ValidationError(c, map[string]string{"password": "Password must be at least 6 characters"})
	}
	if req.Role == "" {
		req.Role = "CUSTOMER"
	}

	var phoneVal *string
	if req.Phone != "" {
		phoneVal = &req.Phone
	}

	result, err := h.service.EmailRegister(c.Context(), req.Email, phoneVal, req.Password, req.FullName, req.Role, h.db != nil)
	if err != nil {
		if strings.HasPrefix(err.Error(), "duplicate:") {
			return response.Error(c, fiber.StatusConflict, "An account with this email or phone number already exists. Please login.", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, fmt.Sprintf("Registration failed: %v", err), nil)
	}

	return response.Created(c, "User registered successfully", fiber.Map{
		"user_id":       result.UserID,
		"email":         req.Email,
		"phone":         req.Phone,
		"full_name":     req.FullName,
		"role":          req.Role,
		"access_token":  result.AccessToken,
		"refresh_token": result.RefToken,
	})
}

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

	result, err := h.service.EmailLogin(c.Context(), req.Email, req.Password)
	if err != nil {
		if strings.Contains(err.Error(), "ACTIVE") || strings.Contains(err.Error(), "contact support") {
			return response.Error(c, fiber.StatusForbidden, err.Error(), nil)
		}
		return response.Error(c, fiber.StatusUnauthorized, "Invalid email or password", nil)
	}

	return response.Success(c, fiber.StatusOK, "Login successful", fiber.Map{
		"access_token":  result.AccessToken,
		"refresh_token": result.RefToken,
		"user_id":       result.UserID,
		"email":         result.Email,
		"phone":         result.Phone,
		"role":          result.Role,
	})
}

func (h *Handler) RefreshToken(c *fiber.Ctx) error {
	type RefreshReq struct {
		RefreshToken string `json:"refresh_token"`
	}
	var req RefreshReq
	if err := c.BodyParser(&req); err != nil || req.RefreshToken == "" {
		return response.BadRequest(c, "refresh_token is required")
	}

	result, err := h.service.RefreshToken(c.Context(), req.RefreshToken, h.db != nil)
	if err != nil {
		return response.Error(c, fiber.StatusUnauthorized, "Invalid or expired refresh token", nil)
	}

	return response.Success(c, fiber.StatusOK, "Token refreshed", fiber.Map{
		"access_token":  result.AccessToken,
		"refresh_token": result.RefToken,
	})
}

func (h *Handler) Logout(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	_ = h.service.Logout(c.Context(), userID)
	return response.Success(c, fiber.StatusOK, "Logged out successfully. All sessions revoked.", nil)
}

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

	result, _ := h.service.PasswordResetRequest(c.Context(), req.Email)

	resp := fiber.Map{"status": "DISPATCHED"}
	if result != nil {
		resp["expires_in"] = "900s"
		resp["reset_otp"] = result.OTPCode
		resp["instructions"] = "Send email + otp_code + new_password to PUT /api/v1/auth/password/reset"
	}

	return response.Success(c, fiber.StatusOK, "If this email is registered, an OTP has been sent.", resp)
}

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

	if err := h.service.PasswordReset(c.Context(), req.Email, req.OTPCode, req.NewPassword); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return response.NotFound(c, err.Error())
		}
		return response.Error(c, fiber.StatusBadRequest, err.Error(), nil)
	}

	return response.Success(c, fiber.StatusOK, "Password reset successfully. All previous sessions have been revoked. Please login with your new password.", fiber.Map{
		"email":            req.Email,
		"status":           "UPDATED",
		"sessions_revoked": true,
	})
}

func (h *Handler) GetSessions(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	sessions, _ := h.service.GetSessions(c.Context(), userID)
	sessionCount := 0
	if sessions != nil {
		sessionCount = len(sessions)
	}
	return response.Success(c, fiber.StatusOK, "Active sessions retrieved", fiber.Map{
		"user_id":  userID,
		"sessions": sessions,
		"count":    sessionCount,
	})
}

func (h *Handler) RevokeSession(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	userID := c.Locals("user_id").(string)

	if err := h.service.RevokeSession(c.Context(), sessionID, userID); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to revoke session", nil)
	}

	return response.Success(c, fiber.StatusOK, "Session revoked successfully", fiber.Map{
		"revoked_session_id": sessionID,
	})
}

func (h *Handler) GetMe(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	user, err := h.service.GetMe(c.Context(), userID)
	if err != nil || user == nil {
		return response.NotFound(c, fmt.Sprintf("User not found in database (id=%s): %v", userID, err))
	}

	return response.Success(c, fiber.StatusOK, "User profile loaded from NeonDB", fiber.Map{
		"id":                  user.ID,
		"email":               user.Email,
		"phone":               user.Phone,
		"full_name":           user.FullName,
		"role":                user.Role,
		"status":              user.Status,
		"email_verified":      user.EmailVerified,
		"phone_verified":      user.PhoneVerified,
		"profile_picture_url": user.ProfilePictureURL,
		"created_at":          user.CreatedAt,
	})
}

func (h *Handler) GoogleCallback(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "OAuth Google Login - configure via OAuth provider", fiber.Map{
		"provider": "google",
		"status":   "not_configured",
	})
}
