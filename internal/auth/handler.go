package auth

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/yourusername/ecom-backend/config"
	"github.com/yourusername/ecom-backend/pkg/middleware"
	"github.com/yourusername/ecom-backend/pkg/response"
)

// Handler wires HTTP routes to the auth Service.
type Handler struct {
	cfg     *config.Config
	db      *pgxpool.Pool
	redis   *redis.Client
	service *Service
}

// NewHandler constructs the auth Handler with all dependencies.
func NewHandler(cfg *config.Config, db *pgxpool.Pool, redisClient *redis.Client) *Handler {
	repo := NewRepository(db)
	service := NewService(repo, cfg, redisClient)
	return &Handler{
		cfg:     cfg,
		db:      db,
		redis:   redisClient,
		service: service,
	}
}

// optionalRateLimit returns a no-op middleware when Redis is unavailable.
func (h *Handler) optionalRateLimit(fn func(*redis.Client) fiber.Handler) fiber.Handler {
	if h.redis == nil {
		return func(c *fiber.Ctx) error { return c.Next() }
	}
	return fn(h.redis)
}

// RegisterRoutes mounts all auth routes on the given router.
func (h *Handler) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	auth := router.Group("/auth")

	otpRL := h.optionalRateLimit(middleware.OTPRateLimit)
	loginRL := h.optionalRateLimit(middleware.LoginRateLimit)

	// Public routes
	auth.Post("/otp/send", otpRL, h.SendOTP)
	auth.Post("/phone/send-otp", otpRL, h.SendOTP) // alias
	auth.Post("/otp/verify", otpRL, h.VerifyOTP)
	auth.Post("/email/login", loginRL, h.EmailLogin)
	auth.Post("/email/register", h.EmailRegister)
	auth.Post("/refresh", h.RefreshToken)
	auth.Post("/password/reset-request", h.PasswordResetRequest)
	auth.Put("/password/reset", h.PasswordReset)

	// Email verification
	auth.Get("/email/verify", h.VerifyEmail)

	// Google OAuth
	auth.Get("/google", h.GoogleAuthRedirect)
	auth.Get("/google/callback", h.GoogleCallback)

	// Authenticated routes
	auth.Post("/logout", authMiddleware, h.Logout)
	auth.Post("/email/resend-verification", authMiddleware, h.ResendVerification)
	auth.Get("/sessions", authMiddleware, h.GetSessions)
	auth.Delete("/sessions/:sessionId", authMiddleware, h.RevokeSession)
	auth.Get("/me", authMiddleware, h.GetMe)
}

// ── Request DTOs ─────────────────────────────────────────────────────────────

type sendOTPReq struct {
	Target  string `json:"target"`
	Phone   string `json:"phone"`
	Purpose string `json:"purpose"`
	Channel string `json:"channel"` // "WHATSAPP", "SMS", "EMAIL", "AUTO"
}

type verifyOTPReq struct {
	Target  string `json:"target"`
	OTPCode string `json:"otp_code"`
	Purpose string `json:"purpose"`
}

type emailRegisterReq struct {
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
	Role     string `json:"role"`
}

type emailLoginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshReq struct {
	RefreshToken string `json:"refresh_token"`
}

type passwordResetRequestReq struct {
	Email string `json:"email"`
}

type passwordResetReq struct {
	Email       string `json:"email"`
	OTPCode     string `json:"otp_code"`
	NewPassword string `json:"new_password"`
}

// ── Handlers ─────────────────────────────────────────────────────────────────

// SendOTP requests an OTP for the given phone/email.
// The OTP is delivered via WhatsApp, SMS or email.
func (h *Handler) SendOTP(c *fiber.Ctx) error {
	var req sendOTPReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request payload")
	}
	if req.Target == "" {
		req.Target = req.Phone
	}
	req.Target = strings.TrimSpace(req.Target)
	if req.Target == "" {
		return response.ValidationError(c, map[string]string{
			"target": "Phone number or email is required",
		})
	}
	if req.Purpose == "" {
		req.Purpose = "LOGIN"
	}

	result, err := h.service.SendOTP(c.Context(), req.Target, req.Purpose, req.Channel)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError,
			"Failed to send OTP: "+err.Error(), nil)
	}

	resMap := fiber.Map{
		"target":         result.Target,
		"expires_in":     result.ExpiresIn,
		"dispatched_via": result.DispatchedVia,
	}
	if result.IsDevMode {
		resMap["dev_otp"] = result.DevOTP
		resMap["dev_note"] = "Development mode: No live gateway configured for this target. OTP is logged to server console."
	}

	msg := fmt.Sprintf("OTP sent successfully via %s to %s", strings.Join(result.DispatchedVia, " & "), result.Target)
	return response.Success(c, fiber.StatusOK, msg, resMap)
}

// VerifyOTP checks the submitted OTP and returns a token pair on success.
func (h *Handler) VerifyOTP(c *fiber.Ctx) error {
	var req verifyOTPReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request payload")
	}
	req.Target = strings.TrimSpace(req.Target)
	req.OTPCode = strings.TrimSpace(req.OTPCode)
	if req.Target == "" || req.OTPCode == "" {
		return response.BadRequest(c, "target and otp_code are required")
	}
	if req.Purpose == "" {
		req.Purpose = "LOGIN"
	}

	currentUserID, _ := c.Locals("user_id").(string)
	result, err := h.service.VerifyOTP(c.Context(), req.Target, req.OTPCode, req.Purpose, currentUserID)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error(), nil)
	}

	return response.Success(c, fiber.StatusOK, "OTP verified — user authenticated", fiber.Map{
		"access_token":  result.AccessToken,
		"refresh_token": result.RefToken,
		"user_id":       result.UserID,
		"phone":         result.Phone,
		"role":          result.Role,
	})
}

// EmailRegister creates a new account with email + password.
func (h *Handler) EmailRegister(c *fiber.Ctx) error {
	var req emailRegisterReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid registration payload")
	}
	req.Email = strings.TrimSpace(req.Email)
	req.Phone = strings.TrimSpace(req.Phone)

	if req.Email == "" || req.Password == "" {
		return response.BadRequest(c, "Email and password are required")
	}
	if !h.service.IsValidEmail(req.Email) {
		return response.ValidationError(c, map[string]string{"email": "Invalid email format"})
	}
	if !h.service.IsStrongPassword(req.Password) {
		return response.ValidationError(c, map[string]string{
			"password": "Password must be at least 8 characters and contain uppercase, lowercase, and a digit",
		})
	}
	if req.Role == "" {
		req.Role = "CUSTOMER"
	}

	var phoneVal *string
	if req.Phone != "" {
		phoneVal = &req.Phone
	}

	result, err := h.service.EmailRegister(c.Context(), req.Email, phoneVal, req.Password, req.FullName, req.Role)
	if err != nil {
		if strings.HasPrefix(err.Error(), "duplicate:") {
			return response.Error(c, fiber.StatusConflict,
				"An account with this email or phone already exists. Please log in.", nil)
		}
		return response.InternalError(c)
	}

	return response.Created(c, "Account created successfully. Please verify your email.", fiber.Map{
		"user_id":       result.UserID,
		"email":         req.Email,
		"phone":         req.Phone,
		"full_name":     req.FullName,
		"role":          req.Role,
		"access_token":  result.AccessToken,
		"refresh_token": result.RefToken,
	})
}

// EmailLogin authenticates with email + password.
func (h *Handler) EmailLogin(c *fiber.Ctx) error {
	var req emailLoginReq
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
		return response.Unauthorized(c, "Invalid email or password")
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

// RefreshToken issues a new token pair via single-use refresh token rotation.
func (h *Handler) RefreshToken(c *fiber.Ctx) error {
	var req refreshReq
	if err := c.BodyParser(&req); err != nil || req.RefreshToken == "" {
		return response.BadRequest(c, "refresh_token is required")
	}

	result, err := h.service.RefreshToken(c.Context(), req.RefreshToken)
	if err != nil {
		return response.Unauthorized(c, "Invalid or expired refresh token")
	}

	return response.Success(c, fiber.StatusOK, "Token refreshed successfully", fiber.Map{
		"access_token":  result.AccessToken,
		"refresh_token": result.RefToken,
	})
}

// Logout revokes all sessions and blacklists the current access token.
func (h *Handler) Logout(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return response.Unauthorized(c, "Authentication required")
	}

	// Blacklist the current access token using its JTI + remaining TTL.
	jti, _ := c.Locals("token_jti").(string)
	ttl, _ := c.Locals("token_ttl").(time.Duration)
	if ttl <= 0 {
		ttl = 15 * time.Minute // fallback to max access token lifetime
	}

	_ = h.service.Logout(c.Context(), userID, jti, ttl)
	return response.Success(c, fiber.StatusOK,
		"Logged out successfully. All sessions have been revoked.", nil)
}

// PasswordResetRequest generates a reset OTP and dispatches it via email.
func (h *Handler) PasswordResetRequest(c *fiber.Ctx) error {
	var req passwordResetRequestReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request payload")
	}
	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" {
		return response.BadRequest(c, "Email is required")
	}

	_, err := h.service.PasswordResetRequest(c.Context(), req.Email)
	if err != nil {
		if strings.Contains(err.Error(), "no account registered") {
			return response.Error(c, fiber.StatusNotFound, err.Error(), nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	return response.Success(c, fiber.StatusOK,
		"Password reset OTP sent successfully to "+req.Email, fiber.Map{
			"email":      req.Email,
			"status":     "DISPATCHED",
			"expires_in": "900s",
		})
}

// PasswordReset verifies the reset OTP and sets the new password.
func (h *Handler) PasswordReset(c *fiber.Ctx) error {
	var req passwordResetReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid password reset payload")
	}
	req.Email = strings.TrimSpace(req.Email)
	req.OTPCode = strings.TrimSpace(req.OTPCode)

	if req.Email == "" || req.OTPCode == "" || req.NewPassword == "" {
		return response.BadRequest(c, "email, otp_code, and new_password are all required")
	}
	if !h.service.IsStrongPassword(req.NewPassword) {
		return response.ValidationError(c, map[string]string{
			"new_password": "Password must be at least 8 characters and contain uppercase, lowercase, and a digit",
		})
	}

	if err := h.service.PasswordReset(c.Context(), req.Email, req.OTPCode, req.NewPassword); err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error(), nil)
	}

	return response.Success(c, fiber.StatusOK,
		"Password reset successfully. All previous sessions have been revoked. Please log in.",
		fiber.Map{"status": "UPDATED", "sessions_revoked": true})
}

// ── Email Verification ────────────────────────────────────────────────────────

// VerifyEmail handles the email verification link click.
// GET /auth/email/verify?token=<raw>&uid=<userID>
func (h *Handler) VerifyEmail(c *fiber.Ctx) error {
	token := strings.TrimSpace(c.Query("token"))
	if token == "" {
		token = strings.TrimSpace(c.Query("amp;token"))
	}
	uid := strings.TrimSpace(c.Query("uid"))
	if uid == "" {
		uid = strings.TrimSpace(c.Query("amp;uid"))
	}

	// Fallback: Parse query parameters directly from OriginalURL (fixes Vercel serverless query string rewriting)
	if token == "" || uid == "" {
		origURL := c.OriginalURL()
		if idx := strings.Index(origURL, "?"); idx != -1 {
			qStr := origURL[idx+1:]
			parsed, err := url.ParseQuery(qStr)
			if err == nil {
				if token == "" {
					token = strings.TrimSpace(parsed.Get("token"))
					if token == "" {
						token = strings.TrimSpace(parsed.Get("amp;token"))
					}
				}
				if uid == "" {
					uid = strings.TrimSpace(parsed.Get("uid"))
					if uid == "" {
						uid = strings.TrimSpace(parsed.Get("amp;uid"))
					}
				}
			}
		}
	}

	wantsJSON := c.Get("Accept") == "application/json" || c.Query("format") == "json"

	if token == "" || uid == "" {
		debugMsg := fmt.Sprintf("Token and user ID are missing. [token='%s', uid='%s', origURL='%s']", token, uid, c.OriginalURL())
		if wantsJSON {
			return response.BadRequest(c, debugMsg)
		}
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.Status(fiber.StatusBadRequest).SendString(renderVerificationResultHTML(false, debugMsg))
	}

	if err := h.service.VerifyEmail(c.Context(), uid, token); err != nil {
		if wantsJSON {
			return response.Error(c, fiber.StatusBadRequest, err.Error(), nil)
		}
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.Status(fiber.StatusBadRequest).SendString(renderVerificationResultHTML(false, err.Error()))
	}

	if wantsJSON {
		return response.Success(c, fiber.StatusOK, "Email verified successfully! You can now access all features.", fiber.Map{
			"email_verified": true,
		})
	}

	c.Set("Content-Type", "text/html; charset=utf-8")
	return c.Status(fiber.StatusOK).SendString(renderVerificationResultHTML(true, "Your email address has been verified successfully! You can now access all features."))
}

func renderVerificationResultHTML(success bool, message string) string {
	title := "Email Verified Successfully!"
	icon := "✅"
	bgColor := "#10B981"
	if !success {
		title = "Verification Failed"
		icon = "❌"
		bgColor = "#EF4444"
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s - MultiVendor E-Commerce</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            background: linear-gradient(135deg, #0F172A 0%%, #1E293B 100%%);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 20px;
            color: #F8FAFC;
        }
        .card {
            background: rgba(30, 41, 59, 0.8);
            border: 1px solid rgba(255, 255, 255, 0.1);
            backdrop-filter: blur(12px);
            border-radius: 24px;
            padding: 48px 36px;
            max-width: 480px;
            width: 100%%;
            text-align: center;
            box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5);
        }
        .icon-box {
            width: 80px;
            height: 80px;
            border-radius: 50%%;
            background: %s20;
            display: flex;
            align-items: center;
            justify-content: center;
            font-size: 40px;
            margin: 0 auto 24px auto;
            border: 2px solid %s;
        }
        h1 { font-size: 24px; font-weight: 700; margin-bottom: 12px; color: #F8FAFC; }
        p { font-size: 15px; color: #94A3B8; line-height: 1.6; margin-bottom: 32px; }
        .btn {
            display: inline-block;
            background: linear-gradient(135deg, #6366F1 0%%, #4F46E5 100%%);
            color: #FFFFFF;
            font-weight: 600;
            font-size: 15px;
            padding: 14px 32px;
            border-radius: 12px;
            text-decoration: none;
            transition: transform 0.2s, box-shadow 0.2s;
            box-shadow: 0 4px 14px 0 rgba(99, 102, 241, 0.39);
        }
        .btn:hover { transform: translateY(-2px); box-shadow: 0 6px 20px 0 rgba(99, 102, 241, 0.5); }
    </style>
</head>
<body>
    <div class="card">
        <div class="icon-box">%s</div>
        <h1>%s</h1>
        <p>%s</p>
        <a href="/" class="btn">Return to Store</a>
    </div>
</body>
</html>`, title, bgColor, bgColor, icon, title, message)
}

// ResendVerification resends the email verification link.
func (h *Handler) ResendVerification(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return response.Unauthorized(c, "Authentication required")
	}

	if err := h.service.ResendVerificationEmail(c.Context(), userID); err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error(), nil)
	}

	return response.Success(c, fiber.StatusOK,
		"Verification email sent. Please check your inbox.", nil)
}

// ── Google OAuth ──────────────────────────────────────────────────────────────

// GoogleAuthRedirect redirects the user to the Google consent page.
// GET /auth/google
func (h *Handler) GoogleAuthRedirect(c *fiber.Ctx) error {
	g := h.service.GoogleOAuthClient()
	if !g.IsConfigured() {
		return response.Error(c, fiber.StatusNotImplemented,
			"Google OAuth is not configured. Set GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET.", nil)
	}

	// Generate and store CSRF state in a short-lived cookie.
	state, err := generateOAuthState()
	if err != nil {
		return response.InternalError(c)
	}

	isSecure := c.Protocol() == "https"
	c.Cookie(&fiber.Cookie{
		Name:     "oauth_state",
		Value:    state,
		MaxAge:   300, // 5 minutes
		HTTPOnly: true,
		Secure:   isSecure,
		SameSite: "Lax",
	})

	return c.Redirect(g.AuthURL(state), fiber.StatusTemporaryRedirect)
}

// GoogleCallback handles the OAuth2 callback from Google.
// GET /auth/google/callback?code=<code>&state=<state>
func (h *Handler) GoogleCallback(c *fiber.Ctx) error {
	g := h.service.GoogleOAuthClient()
	if !g.IsConfigured() {
		return response.Error(c, fiber.StatusNotImplemented,
			"Google OAuth is not configured.", nil)
	}

	isSecure := c.Protocol() == "https"
	// Validate CSRF state.
	state := c.Query("state")
	cookieState := c.Cookies("oauth_state")
	if state == "" || state != cookieState {
		return response.Error(c, fiber.StatusBadRequest,
			"Invalid OAuth state: possible CSRF attack detected", nil)
	}

	// Clear the state cookie.
	c.Cookie(&fiber.Cookie{Name: "oauth_state", Value: "", MaxAge: -1, Secure: isSecure})

	code := c.Query("code")
	if code == "" {
		errDesc := c.Query("error_description", c.Query("error", "authorization denied"))
		return response.Error(c, fiber.StatusBadRequest,
			fmt.Sprintf("OAuth error: %s", errDesc), nil)
	}

	result, err := h.service.HandleGoogleCallback(c.Context(), code)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error(), nil)
	}

	msg := "Google login successful"
	if result.IsNew {
		msg = "Account created via Google — welcome!"
	}

	return response.Success(c, fiber.StatusOK, msg, fiber.Map{
		"access_token":  result.AccessToken,
		"refresh_token": result.RefToken,
		"user_id":       result.UserID,
		"email":         result.Email,
		"name":          result.Name,
		"role":          result.Role,
		"is_new_user":   result.IsNew,
	})
}

// ── Session Management ────────────────────────────────────────────────────────

// GetSessions returns all active sessions for the authenticated user.
func (h *Handler) GetSessions(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return response.Unauthorized(c, "Authentication required")
	}

	sessions, err := h.service.GetSessions(c.Context(), userID)
	if err != nil {
		return response.InternalError(c)
	}

	return response.Success(c, fiber.StatusOK, "Active sessions retrieved", fiber.Map{
		"user_id":  userID,
		"sessions": sessions,
		"count":    len(sessions),
	})
}

// RevokeSession revokes a specific session owned by the authenticated user.
func (h *Handler) RevokeSession(c *fiber.Ctx) error {
	sessionID := strings.TrimPrefix(strings.TrimSpace(c.Params("sessionId")), ":")
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return response.Unauthorized(c, "Authentication required")
	}

	if err := h.service.RevokeSession(c.Context(), sessionID, userID); err != nil {
		return response.InternalError(c)
	}

	return response.Success(c, fiber.StatusOK, "Session revoked successfully", fiber.Map{
		"revoked_session_id": sessionID,
	})
}

// GetMe returns the authenticated user's profile.
func (h *Handler) GetMe(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return response.Unauthorized(c, "Authentication required")
	}

	user, err := h.service.GetMe(c.Context(), userID)
	if err != nil || user == nil {
		return response.NotFound(c, "User")
	}

	return response.Success(c, fiber.StatusOK, "Profile loaded successfully", fiber.Map{
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

// ── Helpers ───────────────────────────────────────────────────────────────────

// generateOAuthState creates a CSRF state token.
func generateOAuthState() (string, error) {
	b := make([]byte, 16)
	if _, err := randRead(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}
