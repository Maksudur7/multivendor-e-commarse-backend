package sqlcdb

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID                       uuid.UUID  `json:"id"`
	Email                    *string    `json:"email"`
	Phone                    *string    `json:"phone"`
	PasswordHash             *string    `json:"password_hash"`
	FullName                 string     `json:"full_name"`
	AvatarURL                *string    `json:"avatar_url"`
	Role                     string     `json:"role"`
	Status                   string     `json:"status"`
	IsEmailVerified          bool       `json:"is_email_verified"`
	IsPhoneVerified          bool       `json:"is_phone_verified"`
	KycStatus                string     `json:"kyc_status"`
	DefaultShippingAddressID *uuid.UUID `json:"default_shipping_address_id"`
	CreatedAt                time.Time  `json:"created_at"`
	UpdatedAt                time.Time  `json:"updated_at"`
}

type OtpSession struct {
	ID        uuid.UUID `json:"id"`
	Target    string    `json:"target"`
	OtpCode   string    `json:"otp_code"`
	Purpose   string    `json:"purpose"`
	IsUsed    bool      `json:"is_used"`
	Attempts  int32     `json:"attempts"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type UserSession struct {
	ID           uuid.UUID `json:"id"`
	UserID       uuid.UUID `json:"user_id"`
	RefreshToken string    `json:"refresh_token"`
	UserAgent    *string   `json:"user_agent"`
	ClientIp     *string   `json:"client_ip"`
	IsRevoked    bool      `json:"is_revoked"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
}
