package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"regexp"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// AuthUser is an alias for the repo DBUser to keep service layer clean
type AuthUser = DBUser

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// --- Pure helpers (no DB) ---

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

func (s *Service) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func (s *Service) CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func (s *Service) IsValidEmail(email string) bool {
	re := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return re.MatchString(email)
}

func (s *Service) BuildAccessToken(userID string) string {
	return fmt.Sprintf("acc_%s_%d", userID, time.Now().Unix())
}

func (s *Service) BuildRefreshToken(userID string) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("ref_%s_%s", userID, hex.EncodeToString(b))
}

func (s *Service) NewUUID() string {
	return uuid.New().String()
}

// --- OTP business operations ---

type SendOTPResult struct {
	OTPCode   string
	ExpiresIn string
}

func (s *Service) SendOTP(ctx context.Context, target, purpose string) (*SendOTPResult, error) {
	otpCode := s.GenerateNumericOTP(6)
	if err := s.repo.InsertOTP(ctx, target, otpCode, purpose, time.Now().Add(5*time.Minute)); err != nil {
		return nil, fmt.Errorf("failed to store OTP: %w", err)
	}
	return &SendOTPResult{OTPCode: otpCode, ExpiresIn: "300s"}, nil
}

type VerifyOTPResult struct {
	UserID      string
	AccessToken string
	RefToken    string
	Phone       string
}

func (s *Service) VerifyOTP(ctx context.Context, target, otpCode, purpose string, dbConnected bool) (*VerifyOTPResult, error) {
	storedOTP, expiresAt, err := s.repo.GetLatestOTP(ctx, target, purpose)
	if err != nil && dbConnected {
		return nil, fmt.Errorf("no OTP found for this target: please request OTP first")
	}
	if dbConnected {
		if time.Now().After(expiresAt) {
			return nil, fmt.Errorf("OTP has expired: please request a new one")
		}
		if storedOTP != otpCode {
			return nil, fmt.Errorf("invalid OTP code")
		}
	}

	userID, err := s.repo.FindOrCreateUserByPhone(ctx, target)
	if err != nil && dbConnected {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	if userID == "" {
		userID = s.NewUUID()
	}

	_ = s.repo.DeleteOTP(ctx, target, purpose)

	accToken := s.BuildAccessToken(userID)
	refToken := s.BuildRefreshToken(userID)
	_ = s.repo.SaveSession(ctx, userID, refToken, time.Now().Add(30*24*time.Hour))

	return &VerifyOTPResult{
		UserID:      userID,
		AccessToken: accToken,
		RefToken:    refToken,
		Phone:       target,
	}, nil
}

// --- Email auth operations ---

type RegisterResult struct {
	UserID      string
	AccessToken string
	RefToken    string
}

func (s *Service) EmailRegister(ctx context.Context, email string, phone *string, password, fullName, role string, dbConnected bool) (*RegisterResult, error) {
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
	if err != nil && dbConnected {
		return nil, fmt.Errorf("registration failed: %w", err)
	}
	if userID == "" {
		userID = s.NewUUID()
	}

	accToken := s.BuildAccessToken(userID)
	refToken := s.BuildRefreshToken(userID)
	_ = s.repo.SaveSession(ctx, userID, refToken, time.Now().Add(30*24*time.Hour))

	return &RegisterResult{UserID: userID, AccessToken: accToken, RefToken: refToken}, nil
}

type LoginResult struct {
	UserID      string
	AccessToken string
	RefToken    string
	Email       string
	Phone       string
	Role        string
}

func (s *Service) EmailLogin(ctx context.Context, email, password string) (*LoginResult, error) {
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

	accToken := s.BuildAccessToken(user.ID)
	refToken := s.BuildRefreshToken(user.ID)
	_ = s.repo.SaveSession(ctx, user.ID, refToken, time.Now().Add(30*24*time.Hour))

	return &LoginResult{
		UserID:      user.ID,
		AccessToken: accToken,
		RefToken:    refToken,
		Email:       email,
		Phone:       user.Phone,
		Role:        user.Role,
	}, nil
}

// --- Token / Session operations ---

type RefreshResult struct {
	AccessToken string
	RefToken    string
}

func (s *Service) RefreshToken(ctx context.Context, oldRefToken string, dbConnected bool) (*RefreshResult, error) {
	userID, isRevoked, err := s.repo.GetSessionByToken(ctx, oldRefToken)
	if (err != nil || isRevoked) && dbConnected {
		return nil, fmt.Errorf("invalid or expired refresh token")
	}
	if userID == "" {
		userID = "unknown"
	}

	newAccToken := s.BuildAccessToken(userID)
	newRefToken := s.BuildRefreshToken(userID)
	_ = s.repo.RevokeSessionByToken(ctx, oldRefToken)
	_ = s.repo.SaveSession(ctx, userID, newRefToken, time.Now().Add(30*24*time.Hour))

	return &RefreshResult{AccessToken: newAccToken, RefToken: newRefToken}, nil
}

func (s *Service) Logout(ctx context.Context, userID string) error {
	return s.repo.RevokeAllUserSessions(ctx, userID)
}

func (s *Service) GetSessions(ctx context.Context, userID string) ([]DBSession, error) {
	return s.repo.GetActiveSessions(ctx, userID)
}

func (s *Service) RevokeSession(ctx context.Context, sessionID, userID string) error {
	return s.repo.RevokeSessionByID(ctx, sessionID, userID)
}

func (s *Service) GetMe(ctx context.Context, userID string) (*DBUser, error) {
	return s.repo.GetUserByID(ctx, userID)
}

// --- Password reset operations ---

type PasswordResetRequestResult struct {
	OTPCode string
}

func (s *Service) PasswordResetRequest(ctx context.Context, email string) (*PasswordResetRequestResult, error) {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil || user == nil {
		// Security: always return success even if email not found
		return nil, nil
	}

	otpCode := s.GenerateNumericOTP(6)
	expiresAt := time.Now().Add(15 * time.Minute)
	_ = s.repo.InsertOTP(ctx, email, otpCode, "PASSWORD_RESET", expiresAt)

	return &PasswordResetRequestResult{OTPCode: otpCode}, nil
}

func (s *Service) PasswordReset(ctx context.Context, email, otpCode, newPassword string) error {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil || user == nil {
		return fmt.Errorf("user with this email not found")
	}

	storedOTP, expiresAt, err := s.repo.GetLatestOTP(ctx, email, "PASSWORD_RESET")
	if err != nil {
		return fmt.Errorf("no password reset request found: please request OTP first")
	}
	if time.Now().After(expiresAt) {
		return fmt.Errorf("OTP has expired: please request a new one")
	}
	if storedOTP != otpCode {
		return fmt.Errorf("invalid OTP code")
	}

	passHash, err := s.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to secure new password: %w", err)
	}

	return s.repo.UpdatePasswordAndRevoke(ctx, user.ID, passHash, email)
}
