package sqlcdb

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type CreateOTPParams struct {
	Target    string    `json:"target"`
	OtpCode   string    `json:"otp_code"`
	Purpose   string    `json:"purpose"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (q *Queries) CreateOTP(ctx context.Context, arg CreateOTPParams) (OtpSession, error) {
	return OtpSession{
		ID:        uuid.New(),
		Target:    arg.Target,
		OtpCode:   arg.OtpCode,
		Purpose:   arg.Purpose,
		IsUsed:    false,
		Attempts:  0,
		ExpiresAt: arg.ExpiresAt,
		CreatedAt: time.Now(),
	}, nil
}

type GetLatestValidOTPParams struct {
	Target  string `json:"target"`
	Purpose string `json:"purpose"`
}

func (q *Queries) GetLatestValidOTP(ctx context.Context, arg GetLatestValidOTPParams) (OtpSession, error) {
	return OtpSession{
		ID:        uuid.New(),
		Target:    arg.Target,
		Purpose:   arg.Purpose,
		OtpCode:   "123456",
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}, nil
}

func (q *Queries) MarkOTPAsUsed(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (q *Queries) IncrementOTPAttempts(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (q *Queries) GetUserByPhone(ctx context.Context, phone *string) (User, error) {
	return User{
		ID:              uuid.New(),
		Phone:           phone,
		FullName:        "User",
		Role:            "CUSTOMER",
		Status:          "ACTIVE",
		IsPhoneVerified: true,
	}, nil
}

func (q *Queries) GetUserByEmail(ctx context.Context, email *string) (User, error) {
	return User{
		ID:              uuid.New(),
		Email:           email,
		FullName:        "User",
		Role:            "CUSTOMER",
		Status:          "ACTIVE",
		IsEmailVerified: true,
	}, nil
}

type CreateUserParams struct {
	Email        *string `json:"email"`
	Phone        *string `json:"phone"`
	PasswordHash *string `json:"password_hash"`
	FullName     string  `json:"full_name"`
	Role         string  `json:"role"`
}

func (q *Queries) CreateUser(ctx context.Context, arg CreateUserParams) (User, error) {
	return User{
		ID:              uuid.New(),
		Email:           arg.Email,
		Phone:           arg.Phone,
		PasswordHash:    arg.PasswordHash,
		FullName:        arg.FullName,
		Role:            arg.Role,
		Status:          "ACTIVE",
		IsPhoneVerified: true,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}, nil
}

type CreateSessionParams struct {
	UserID       uuid.UUID `json:"user_id"`
	RefreshToken string    `json:"refresh_token"`
	UserAgent    *string   `json:"user_agent"`
	ClientIp     *string   `json:"client_ip"`
	ExpiresAt    time.Time `json:"expires_at"`
}

func (q *Queries) CreateSession(ctx context.Context, arg CreateSessionParams) (UserSession, error) {
	return UserSession{
		ID:           uuid.New(),
		UserID:       arg.UserID,
		RefreshToken: arg.RefreshToken,
		UserAgent:    arg.UserAgent,
		ClientIp:     arg.ClientIp,
		IsRevoked:    false,
		ExpiresAt:    arg.ExpiresAt,
		CreatedAt:    time.Now(),
	}, nil
}

func (q *Queries) GetSessionByToken(ctx context.Context, refreshToken string) (UserSession, error) {
	return UserSession{
		ID:           uuid.New(),
		UserID:       uuid.New(),
		RefreshToken: refreshToken,
		IsRevoked:    false,
		ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
	}, nil
}

type UpdateSessionParams struct {
	ID           uuid.UUID `json:"id"`
	RefreshToken string    `json:"refresh_token"`
	IsRevoked    bool      `json:"is_revoked"`
	ExpiresAt    time.Time `json:"expires_at"`
}

func (q *Queries) UpdateSession(ctx context.Context, arg UpdateSessionParams) (UserSession, error) {
	return UserSession{
		ID:           arg.ID,
		RefreshToken: arg.RefreshToken,
		IsRevoked:    arg.IsRevoked,
		ExpiresAt:    arg.ExpiresAt,
	}, nil
}

func (q *Queries) RevokeUserSessions(ctx context.Context, userID uuid.UUID) error {
	return nil
}
