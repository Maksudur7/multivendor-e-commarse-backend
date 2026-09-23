package sqlcdb

import (
	"context"

	"github.com/google/uuid"
)

type Querier interface {
	CreateOTP(ctx context.Context, arg CreateOTPParams) (OtpSession, error)
	GetLatestValidOTP(ctx context.Context, arg GetLatestValidOTPParams) (OtpSession, error)
	MarkOTPAsUsed(ctx context.Context, id uuid.UUID) error
	IncrementOTPAttempts(ctx context.Context, id uuid.UUID) error
	GetUserByPhone(ctx context.Context, phone *string) (User, error)
	GetUserByEmail(ctx context.Context, email *string) (User, error)
	CreateUser(ctx context.Context, arg CreateUserParams) (User, error)
	CreateSession(ctx context.Context, arg CreateSessionParams) (UserSession, error)
	GetSessionByToken(ctx context.Context, refreshToken string) (UserSession, error)
	UpdateSession(ctx context.Context, arg UpdateSessionParams) (UserSession, error)
	RevokeUserSessions(ctx context.Context, userID uuid.UUID) error
}

var _ Querier = (*Queries)(nil)
