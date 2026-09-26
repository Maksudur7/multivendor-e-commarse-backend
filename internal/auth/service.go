package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"github.com/yourusername/ecom-backend/config"
	pkgemail "github.com/yourusername/ecom-backend/pkg/email"
	"github.com/yourusername/ecom-backend/pkg/oauth"
	pkgsms "github.com/yourusername/ecom-backend/pkg/sms"
	pkgwhatsapp "github.com/yourusername/ecom-backend/pkg/whatsapp"
)

// AuthUser is an alias for the repo DBUser to keep service layer clean.
type AuthUser = DBUser

// JWTClaims is the verified JWT payload embedded in every access token.
type JWTClaims struct {
	UserID    string `json:"sub"`
	Role      string `json:"role"`
	SessionID string `json:"session_id"`
	jwt.RegisteredClaims
}

// Service holds all business-logic for the auth domain.
type Service struct {
	repo             *Repository
	jwtAccessSecret  string
	jwtAccessExpiry  time.Duration
	jwtRefreshExpiry time.Duration
	redis            *redis.Client
	sms              *pkgsms.Client
	whatsapp         *pkgwhatsapp.Client
	email            *pkgemail.Client
	google           *oauth.GoogleClient
	appBaseURL       string
}

// NewService constructs a Service.
func NewService(
	repo *Repository,
	cfg *config.Config,
	redisClient *redis.Client,
) *Service {
	return &Service{
		repo:             repo,
		jwtAccessSecret:  cfg.JWT.AccessSecret,
		jwtAccessExpiry:  cfg.JWT.AccessExpiry,
		jwtRefreshExpiry: cfg.JWT.RefreshExpiry,
		redis:            redisClient,
		sms:              pkgsms.NewClient(cfg.SMS.APIKey, cfg.SMS.SenderID),
		whatsapp:         pkgwhatsapp.NewClient(cfg.WhatsApp.InstanceID, cfg.WhatsApp.Token),
		email:            pkgemail.NewClient(cfg.Email),
		google:           oauth.NewGoogleClient(cfg.Google.ClientID, cfg.Google.ClientSecret, cfg.Google.RedirectURL),
		appBaseURL:       cfg.App.BaseURL,
	}
}

// ── Cryptographic helpers ────────────────────────────────────────────────────

// HashSHA256 hashes s with SHA-256 and returns the hex string.
func HashSHA256(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// generateSecureRandom produces n cryptographically random bytes as hex string.
func generateSecureRandom(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto/rand failed: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// GenerateNumericOTP generates a cryptographically-random numeric OTP.
func (s *Service) GenerateNumericOTP(length int) string {
	const digits = "0123456789"
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			result[i] = digits[i%10]
		} else {
			result[i] = digits[num.Int64()]
		}
	}
	return string(result)
}

// HashPassword bcrypt-hashes a plaintext password.
func (s *Service) HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(b), err
}

// CheckPassword compares a plaintext password against its bcrypt hash.
func (s *Service) CheckPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// IsValidEmail validates the email format.
func (s *Service) IsValidEmail(email string) bool {
	re := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return re.MatchString(email)
}

// IsStrongPassword enforces: ≥8 chars, at least 1 uppercase, 1 lowercase, 1 digit.
func (s *Service) IsStrongPassword(password string) bool {
	if len(password) < 8 {
		return false
	}
	var hasUpper, hasLower, hasDigit bool
	for _, c := range password {
		switch {
		case unicode.IsUpper(c):
			hasUpper = true
		case unicode.IsLower(c):
			hasLower = true
		case unicode.IsDigit(c):
			hasDigit = true
		}
	}
	return hasUpper && hasLower && hasDigit
}

// NewUUID returns a new random UUID string.
func (s *Service) NewUUID() string {
	return uuid.New().String()
}

// GoogleOAuthClient exposes the Google client for use in the handler.
func (s *Service) GoogleOAuthClient() *oauth.GoogleClient {
	return s.google
}

// ── Token builders ───────────────────────────────────────────────────────────

// BuildAccessToken signs a JWT with HS256.
func (s *Service) BuildAccessToken(userID, role, sessionID string) (string, error) {
	now := time.Now()
	claims := JWTClaims{
		UserID:    userID,
		Role:      role,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.jwtAccessExpiry)),
			NotBefore: jwt.NewNumericDate(now),
			ID:        uuid.New().String(), // jti — unique per token
			Issuer:    "ecom-backend",
			Audience:  jwt.ClaimStrings{"ecom-api"},
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.jwtAccessSecret))
	if err != nil {
		return "", fmt.Errorf("failed to sign access token: %w", err)
	}
	return signed, nil
}

// BuildRefreshToken generates a 32-byte cryptographically random refresh token.
// Returns raw token (for client) and its SHA-256 hash (for DB storage).
func (s *Service) BuildRefreshToken() (rawToken, hashedToken string, err error) {
	raw, err := generateSecureRandom(32)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate refresh token: %w", err)
	}
	return raw, HashSHA256(raw), nil
}

// ── Redis Token Blacklist ────────────────────────────────────────────────────

const blacklistKeyPrefix = "blacklist:jti:"

// BlacklistToken adds a JWT's jti to the Redis blacklist.
// ttl should be the remaining lifetime of the token.
func (s *Service) BlacklistToken(ctx context.Context, jti string, ttl time.Duration) error {
	if s.redis == nil || jti == "" {
		return nil
	}
	key := blacklistKeyPrefix + jti
	return s.redis.Set(ctx, key, "1", ttl).Err()
}

// IsTokenBlacklisted checks if a JWT's jti is in the Redis blacklist.
func (s *Service) IsTokenBlacklisted(ctx context.Context, jti string) bool {
	if s.redis == nil || jti == "" {
		return false
	}
	exists, err := s.redis.Exists(ctx, blacklistKeyPrefix+jti).Result()
	return err == nil && exists > 0
}

// ── OTP business operations ──────────────────────────────────────────────────

// SendOTPResult is returned by SendOTP.
type SendOTPResult struct {
	Target        string   `json:"target"`
	ExpiresIn     int      `json:"expires_in"` // seconds
	DispatchedVia []string `json:"dispatched_via"`
	DevOTP        string   `json:"dev_otp,omitempty"`
	IsDevMode     bool     `json:"is_dev_mode"`
}

// SendOTP generates a 6-digit OTP, SHA-256 hashes it, stores the hash,
// and dispatches the raw OTP via WhatsApp, SMS (for phone) or email.
// channel optional: "WHATSAPP", "SMS", "AUTO" / ""
func (s *Service) SendOTP(ctx context.Context, target, purpose, channel string) (*SendOTPResult, error) {
	if s.repo.db == nil {
		return nil, fmt.Errorf("service unavailable: database not connected")
	}

	// Normalize phone number if target is a phone number
	if pkgsms.IsPhone(target) {
		target = pkgsms.NormalizePhone(target)
	}

	otpCode := s.GenerateNumericOTP(6)
	hashedOTP := HashSHA256(otpCode)

	if err := s.repo.InsertOTP(ctx, target, hashedOTP, purpose, time.Now().Add(5*time.Minute)); err != nil {
		return nil, fmt.Errorf("failed to store OTP: %w", err)
	}

	var dispatchedVia []string
	channelUpper := strings.ToUpper(strings.TrimSpace(channel))
	isDevMode := false

	if pkgsms.IsPhone(target) {
		waLive := s.whatsapp.IsConfigured()
		smsLive := s.sms.IsConfigured()

		var sendErr error

		// Dispatch WhatsApp if requested or AUTO (if live)
		if channelUpper == "WHATSAPP" || channelUpper == "" || channelUpper == "AUTO" || channelUpper == "ALL" {
			if waLive {
				if err := s.whatsapp.SendOTP(ctx, target, otpCode); err != nil {
					sendErr = err
				} else {
					dispatchedVia = append(dispatchedVia, "whatsapp")
				}
			}
		}

		// Dispatch SMS if requested or AUTO (if live)
		if channelUpper == "SMS" || channelUpper == "" || channelUpper == "AUTO" || channelUpper == "ALL" {
			if smsLive {
				if err := s.sms.SendOTP(ctx, target, otpCode); err != nil {
					sendErr = err
				} else {
					dispatchedVia = append(dispatchedVia, "sms")
				}
			}
		}

		// Explicit channel validation & error handling
		if channelUpper == "WHATSAPP" && !waLive {
			return nil, fmt.Errorf("WhatsApp gateway (UltraMsg) is not configured with live API credentials")
		}
		if channelUpper == "SMS" && !smsLive {
			return nil, fmt.Errorf("SMS gateway (Greenweb) is not configured with live API credentials")
		}

		// If no live gateways dispatched, fall back to dev mode
		if len(dispatchedVia) == 0 {
			if sendErr != nil {
				return nil, fmt.Errorf("failed to send OTP: %w", sendErr)
			}
			isDevMode = true
			dispatchedVia = append(dispatchedVia, "dev_console")
			_ = s.sms.SendOTP(ctx, target, otpCode)
			_ = s.whatsapp.SendOTP(ctx, target, otpCode)
		}
	} else {
		if err := s.email.SendOTP(ctx, target, otpCode); err != nil {
			return nil, fmt.Errorf("failed to send Email OTP to %s: %w", target, err)
		}
		dispatchedVia = append(dispatchedVia, "email")
	}

	res := &SendOTPResult{
		Target:        target,
		ExpiresIn:     300,
		DispatchedVia: dispatchedVia,
		IsDevMode:     isDevMode,
	}
	if isDevMode {
		res.DevOTP = otpCode
	}

	return res, nil
}

// maxOTPAttempts is the maximum number of wrong OTP guesses before lockout.
const maxOTPAttempts = 5

// VerifyOTPResult is returned by VerifyOTP on success.
type VerifyOTPResult struct {
	UserID      string
	AccessToken string
	RefToken    string
	Phone       string
	Role        string
}

// VerifyOTP verifies the submitted OTP against the stored hash.
// After maxOTPAttempts wrong guesses, the OTP is invalidated (brute-force lockout).
func (s *Service) VerifyOTP(ctx context.Context, target, otpCode, purpose string) (*VerifyOTPResult, error) {
	if s.repo.db == nil {
		return nil, fmt.Errorf("service unavailable: database not connected")
	}

	// Normalize phone number if target is a phone number
	if pkgsms.IsPhone(target) {
		target = pkgsms.NormalizePhone(target)
	}

	// Check attempt count BEFORE verifying (brute-force lockout).
	attempts, err := s.repo.GetOTPAttemptCount(ctx, target, purpose)
	if err == nil && attempts >= maxOTPAttempts {
		// Invalidate the OTP so a new one must be requested.
		_ = s.repo.DeleteOTP(ctx, target, purpose)
		return nil, fmt.Errorf("too many incorrect attempts: OTP has been invalidated. Please request a new one")
	}

	hashedInput := HashSHA256(otpCode)
	storedHash, expiresAt, err := s.repo.GetLatestOTP(ctx, target, purpose)
	if err != nil {
		return nil, fmt.Errorf("no OTP found for this target: please request OTP first")
	}
	if time.Now().After(expiresAt) {
		return nil, fmt.Errorf("OTP has expired: please request a new one")
	}
	if storedHash != hashedInput {
		_ = s.repo.IncrementOTPAttempt(ctx, target, purpose)
		remaining := maxOTPAttempts - attempts - 1
		if remaining <= 0 {
			_ = s.repo.DeleteOTP(ctx, target, purpose)
			return nil, fmt.Errorf("invalid OTP: maximum attempts reached. Please request a new OTP")
		}
		return nil, fmt.Errorf("invalid OTP code: %d attempts remaining", remaining)
	}

	// OTP valid — find or create the user.
	userID, err := s.repo.FindOrCreateUserByPhone(ctx, target)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Consume the OTP so it cannot be reused.
	_ = s.repo.DeleteOTP(ctx, target, purpose)

	// Issue token pair.
	sessionID := s.NewUUID()
	accToken, err := s.BuildAccessToken(userID, "CUSTOMER", sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to build access token: %w", err)
	}
	rawRefToken, hashedRefToken, err := s.BuildRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed to build refresh token: %w", err)
	}
	if err := s.repo.SaveSession(ctx, userID, hashedRefToken, time.Now().Add(s.jwtRefreshExpiry)); err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	return &VerifyOTPResult{
		UserID:      userID,
		AccessToken: accToken,
		RefToken:    rawRefToken,
		Phone:       target,
		Role:        "CUSTOMER",
	}, nil
}

// ── Email auth operations ────────────────────────────────────────────────────

// RegisterResult is returned by EmailRegister.
type RegisterResult struct {
	UserID      string
	AccessToken string
	RefToken    string
}

// EmailRegister creates a new user account with email/password and sends a verification email.
func (s *Service) EmailRegister(ctx context.Context, email string, phone *string, password, fullName, role string) (*RegisterResult, error) {
	if s.repo.db == nil {
		return nil, fmt.Errorf("service unavailable: database not connected")
	}

	phoneStr := ""
	if phone != nil {
		phoneStr = *phone
	}
	if s.repo.CheckDuplicateAccount(ctx, email, phoneStr) {
		return nil, fmt.Errorf("duplicate: an account with this email or phone number already exists")
	}

	passHash, err := s.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to secure password: %w", err)
	}

	userID, err := s.repo.CreateEmailUser(ctx, email, phone, passHash, fullName, role)
	if err != nil {
		return nil, fmt.Errorf("registration failed: %w", err)
	}

	sessionID := s.NewUUID()
	accToken, err := s.BuildAccessToken(userID, role, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to build access token: %w", err)
	}
	rawRefToken, hashedRefToken, err := s.BuildRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed to build refresh token: %w", err)
	}
	if err := s.repo.SaveSession(ctx, userID, hashedRefToken, time.Now().Add(s.jwtRefreshExpiry)); err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	// Send email verification asynchronously.
	go s.sendEmailVerificationLink(context.Background(), userID, email, fullName)

	return &RegisterResult{UserID: userID, AccessToken: accToken, RefToken: rawRefToken}, nil
}

// sendEmailVerificationLink generates a token and sends the verification email.
func (s *Service) sendEmailVerificationLink(ctx context.Context, userID, email, fullName string) {
	token, err := generateSecureRandom(32)
	if err != nil {
		fmt.Printf("[EMAIL-ERR] Failed to generate verification token for %s: %v\n", email, err)
		return
	}
	hashedToken := HashSHA256(token)
	expiresAt := time.Now().Add(24 * time.Hour)

	if err := s.repo.SaveEmailVerificationToken(ctx, userID, hashedToken, expiresAt); err != nil {
		fmt.Printf("[EMAIL-ERR] Failed to save verification token for %s (userID=%s): %v\n", email, userID, err)
		// Still try to send the email even if token storage fails
	}

	link := fmt.Sprintf("%s/api/v1/auth/email/verify?token=%s&uid=%s",
		s.appBaseURL, token, userID)
	if err := s.email.SendVerificationEmail(ctx, email, fullName, link); err != nil {
		fmt.Printf("[EMAIL-ERR] Verification email failed to %s: %v\n", email, err)
	} else {
		fmt.Printf("[EMAIL-OK] Verification email sent to %s\n", email)
	}
}

// LoginResult is returned by EmailLogin.
type LoginResult struct {
	UserID      string
	AccessToken string
	RefToken    string
	Email       string
	Phone       string
	Role        string
}

// EmailLogin authenticates with email + password.
func (s *Service) EmailLogin(ctx context.Context, email, password string) (*LoginResult, error) {
	if s.repo.db == nil {
		return nil, fmt.Errorf("service unavailable: database not connected")
	}

	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil || user == nil {
		return nil, fmt.Errorf("invalid email or password")
	}
	if user.Status != "ACTIVE" {
		return nil, fmt.Errorf("account is %s: please contact support", user.Status)
	}
	if !s.CheckPassword(password, user.PasswordHash) {
		return nil, fmt.Errorf("invalid email or password")
	}

	sessionID := s.NewUUID()
	accToken, err := s.BuildAccessToken(user.ID, user.Role, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to build access token: %w", err)
	}
	rawRefToken, hashedRefToken, err := s.BuildRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed to build refresh token: %w", err)
	}
	if err := s.repo.SaveSession(ctx, user.ID, hashedRefToken, time.Now().Add(s.jwtRefreshExpiry)); err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	return &LoginResult{
		UserID:      user.ID,
		AccessToken: accToken,
		RefToken:    rawRefToken,
		Email:       email,
		Phone:       user.Phone,
		Role:        user.Role,
	}, nil
}

// ── Token / Session operations ───────────────────────────────────────────────

// RefreshResult is returned by RefreshToken.
type RefreshResult struct {
	AccessToken string
	RefToken    string
}

// RefreshToken validates the raw refresh token, rotates it, and issues a new pair.
func (s *Service) RefreshToken(ctx context.Context, oldRawRefToken string) (*RefreshResult, error) {
	if s.repo.db == nil {
		return nil, fmt.Errorf("service unavailable: database not connected")
	}

	hashedOld := HashSHA256(oldRawRefToken)
	userID, role, isRevoked, err := s.repo.GetSessionByToken(ctx, hashedOld)
	if err != nil || isRevoked {
		return nil, fmt.Errorf("invalid or expired refresh token")
	}

	// Revoke consumed token immediately.
	_ = s.repo.RevokeSessionByToken(ctx, hashedOld)

	newSessionID := s.NewUUID()
	newAccToken, err := s.BuildAccessToken(userID, role, newSessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to build access token: %w", err)
	}
	rawRefToken, hashedRefToken, err := s.BuildRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed to build refresh token: %w", err)
	}
	_ = s.repo.SaveSession(ctx, userID, hashedRefToken, time.Now().Add(s.jwtRefreshExpiry))

	return &RefreshResult{AccessToken: newAccToken, RefToken: rawRefToken}, nil
}

// Logout revokes all active sessions and blacklists the current access token.
func (s *Service) Logout(ctx context.Context, userID, jti string, tokenTTL time.Duration) error {
	// Blacklist the current access token so it can't be used until expiry.
	_ = s.BlacklistToken(ctx, jti, tokenTTL)
	// Revoke all refresh sessions.
	return s.repo.RevokeAllUserSessions(ctx, userID)
}

// GetSessions returns all non-expired, non-revoked sessions.
func (s *Service) GetSessions(ctx context.Context, userID string) ([]DBSession, error) {
	return s.repo.GetActiveSessions(ctx, userID)
}

// RevokeSession revokes a specific session owned by userID.
func (s *Service) RevokeSession(ctx context.Context, sessionID, userID string) error {
	return s.repo.RevokeSessionByID(ctx, sessionID, userID)
}

// GetMe fetches the full user profile.
func (s *Service) GetMe(ctx context.Context, userID string) (*DBUser, error) {
	return s.repo.GetUserByID(ctx, userID)
}

// ── Email Verification ───────────────────────────────────────────────────────

// VerifyEmail validates the email verification token and marks the user verified.
func (s *Service) VerifyEmail(ctx context.Context, userID, rawToken string) error {
	if s.repo.db == nil {
		return fmt.Errorf("service unavailable: database not connected")
	}
	hashedToken := HashSHA256(rawToken)
	return s.repo.VerifyEmail(ctx, userID, hashedToken)
}

// ResendVerificationEmail re-sends the email verification link.
func (s *Service) ResendVerificationEmail(ctx context.Context, userID string) error {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil || user == nil {
		return fmt.Errorf("user not found")
	}
	if user.EmailVerified {
		return fmt.Errorf("email is already verified")
	}
	go s.sendEmailVerificationLink(context.Background(), userID, user.Email, user.FullName)
	return nil
}

// ── Password reset operations ────────────────────────────────────────────────

// PasswordResetRequestResult is returned by PasswordResetRequest.
type PasswordResetRequestResult struct{}

// PasswordResetRequest generates a reset OTP and sends it via email.
func (s *Service) PasswordResetRequest(ctx context.Context, email string) (*PasswordResetRequestResult, error) {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil || user == nil {
		return nil, fmt.Errorf("no account registered with email address: %s", email)
	}

	otpCode := s.GenerateNumericOTP(6)
	hashedOTP := HashSHA256(otpCode)
	expiresAt := time.Now().Add(15 * time.Minute)
	if err := s.repo.InsertOTP(ctx, email, hashedOTP, "PASSWORD_RESET", expiresAt); err != nil {
		return nil, fmt.Errorf("failed to save reset OTP: %w", err)
	}

	// Send password-reset OTP via email.
	if err := s.email.SendPasswordResetOTP(ctx, email, otpCode); err != nil {
		return nil, fmt.Errorf("failed to send password reset email to %s: %w", email, err)
	}

	return &PasswordResetRequestResult{}, nil
}

// PasswordReset verifies the OTP and sets the new password.
func (s *Service) PasswordReset(ctx context.Context, email, otpCode, newPassword string) error {
	if s.repo.db == nil {
		return fmt.Errorf("service unavailable: database not connected")
	}

	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil || user == nil {
		return fmt.Errorf("invalid request: check your email and OTP")
	}

	hashedInput := HashSHA256(otpCode)
	storedHash, expiresAt, err := s.repo.GetLatestOTP(ctx, email, "PASSWORD_RESET")
	if err != nil {
		return fmt.Errorf("no password reset request found: please request OTP first")
	}
	if time.Now().After(expiresAt) {
		return fmt.Errorf("OTP has expired: please request a new one")
	}
	if storedHash != hashedInput {
		return fmt.Errorf("invalid OTP code")
	}

	passHash, err := s.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to secure new password: %w", err)
	}

	return s.repo.UpdatePasswordAndRevoke(ctx, user.ID, passHash, email)
}

// ── Google OAuth operations ──────────────────────────────────────────────────

// GoogleLoginResult is returned after a successful Google OAuth login.
type GoogleLoginResult struct {
	UserID      string
	AccessToken string
	RefToken    string
	Email       string
	Name        string
	Role        string
	IsNew       bool
}

// HandleGoogleCallback processes the OAuth callback, finds or creates the user,
// and issues a token pair.
func (s *Service) HandleGoogleCallback(ctx context.Context, code string) (*GoogleLoginResult, error) {
	if s.repo.db == nil {
		return nil, fmt.Errorf("service unavailable: database not connected")
	}
	if !s.google.IsConfigured() {
		return nil, fmt.Errorf("Google OAuth is not configured")
	}

	// Exchange authorization code for tokens.
	token, err := s.google.ExchangeCode(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("OAuth code exchange failed: %w", err)
	}

	// Fetch user info from Google.
	info, err := s.google.GetUserInfo(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("failed to get Google user info: %w", err)
	}

	// Find or create user by Google ID / email.
	userID, isNew, err := s.repo.FindOrCreateGoogleUser(ctx, info.ID, info.Email, info.Name, info.Picture)
	if err != nil {
		return nil, fmt.Errorf("failed to find or create user: %w", err)
	}

	// Get user's role.
	role := "CUSTOMER"
	if !isNew {
		if u, err := s.repo.GetUserByID(ctx, userID); err == nil && u != nil {
			role = u.Role
		}
	}

	sessionID := s.NewUUID()
	accToken, err := s.BuildAccessToken(userID, role, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to build access token: %w", err)
	}
	rawRefToken, hashedRefToken, err := s.BuildRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed to build refresh token: %w", err)
	}
	_ = s.repo.SaveSession(ctx, userID, hashedRefToken, time.Now().Add(s.jwtRefreshExpiry))

	return &GoogleLoginResult{
		UserID:      userID,
		AccessToken: accToken,
		RefToken:    rawRefToken,
		Email:       info.Email,
		Name:        info.Name,
		Role:        role,
		IsNew:       isNew,
	}, nil
}
