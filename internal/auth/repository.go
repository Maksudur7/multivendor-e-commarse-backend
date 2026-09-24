package auth

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) InsertOTP(ctx context.Context, target, otpCode, purpose string, expiresAt time.Time) error {
	if r.db == nil {
		return nil
	}
	_, err := r.db.Exec(ctx,
		"INSERT INTO otp_requests (phone, otp_hash, purpose, expires_at) VALUES ($1, $2, $3, $4)",
		target, otpCode, purpose, expiresAt,
	)
	return err
}

func (r *Repository) GetLatestOTP(ctx context.Context, target, purpose string) (string, time.Time, error) {
	var storedOTP string
	var expiresAt time.Time
	if r.db == nil {
		return "", time.Now(), nil
	}
	err := r.db.QueryRow(ctx,
		`SELECT otp_hash, expires_at FROM otp_requests
		 WHERE phone = $1 AND purpose = $2
		 ORDER BY created_at DESC LIMIT 1`,
		target, purpose,
	).Scan(&storedOTP, &expiresAt)
	return storedOTP, expiresAt, err
}

func (r *Repository) DeleteOTP(ctx context.Context, target, purpose string) error {
	if r.db == nil {
		return nil
	}
	_, err := r.db.Exec(ctx, "DELETE FROM otp_requests WHERE phone = $1 AND purpose = $2", target, purpose)
	return err
}

func (r *Repository) FindOrCreateUserByPhone(ctx context.Context, phone string) (string, error) {
	if r.db == nil {
		return "", nil
	}
	var existingID string
	err := r.db.QueryRow(ctx, "SELECT id::text FROM users WHERE phone = $1", phone).Scan(&existingID)
	if err == nil {
		return existingID, nil
	}

	var newID string
	err = r.db.QueryRow(ctx,
		"INSERT INTO users (phone, full_name, role, status) VALUES ($1, $2, 'CUSTOMER', 'ACTIVE') RETURNING id::text",
		phone, "User-"+phone[len(phone)-4:],
	).Scan(&newID)
	return newID, err
}

func (r *Repository) CheckDuplicateAccount(ctx context.Context, email, phone string) bool {
	if r.db == nil {
		return false
	}
	var existCheck string
	err := r.db.QueryRow(ctx, "SELECT id::text FROM users WHERE email = $1 OR (phone IS NOT NULL AND phone = $2 AND $2 != '')", email, phone).Scan(&existCheck)
	return err == nil
}

func (r *Repository) CreateEmailUser(ctx context.Context, email string, phoneVal *string, passHash, fullName, role string) (string, error) {
	if r.db == nil {
		return "", nil
	}
	var userID string
	err := r.db.QueryRow(ctx,
		`INSERT INTO users (email, phone, password_hash, full_name, role, status, email_verified, phone_verified)
		 VALUES ($1, $2, $3, $4, $5, 'ACTIVE', true, true)
		 RETURNING id::text`,
		email, phoneVal, passHash, fullName, role,
	).Scan(&userID)
	return userID, err
}

func (r *Repository) SaveSession(ctx context.Context, userID, refreshToken string, expiresAt time.Time) error {
	if r.db == nil {
		return nil
	}
	_, err := r.db.Exec(ctx,
		"INSERT INTO sessions (user_id, refresh_token_hash, expires_at, is_revoked) VALUES ($1, $2, $3, false)",
		userID, refreshToken, expiresAt,
	)
	return err
}

type UserLoginRecord struct {
	ID           string
	PasswordHash string
	Role         string
	Status       string
	Phone        string
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*UserLoginRecord, error) {
	if r.db == nil {
		return nil, nil
	}
	var id, storedHash, role, status string
	var phone *string
	err := r.db.QueryRow(ctx,
		"SELECT id::text, password_hash, role, status, phone FROM users WHERE email = $1",
		email,
	).Scan(&id, &storedHash, &role, &status, &phone)
	if err != nil {
		return nil, err
	}
	phoneStr := ""
	if phone != nil { phoneStr = *phone }
	return &UserLoginRecord{
		ID:           id,
		PasswordHash: storedHash,
		Role:         role,
		Status:       status,
		Phone:        phoneStr,
	}, nil
}

func (r *Repository) GetSessionByToken(ctx context.Context, refreshToken string) (string, bool, error) {
	if r.db == nil {
		return "", false, nil
	}
	var userID string
	var isRevoked bool
	err := r.db.QueryRow(ctx,
		"SELECT user_id::text, is_revoked FROM sessions WHERE refresh_token_hash = $1",
		refreshToken,
	).Scan(&userID, &isRevoked)
	return userID, isRevoked, err
}

func (r *Repository) RevokeSessionByToken(ctx context.Context, refreshToken string) error {
	if r.db == nil {
		return nil
	}
	_, err := r.db.Exec(ctx, "UPDATE sessions SET is_revoked = true WHERE refresh_token_hash = $1", refreshToken)
	return err
}

func (r *Repository) RevokeAllUserSessions(ctx context.Context, userID string) error {
	if r.db == nil {
		return nil
	}
	_, err := r.db.Exec(ctx, "UPDATE sessions SET is_revoked = true WHERE user_id::text = $1 AND is_revoked = false", userID)
	return err
}

func (r *Repository) UpdatePasswordAndRevoke(ctx context.Context, userID, passHash, email string) error {
	if r.db == nil {
		return nil
	}
	_, err := r.db.Exec(ctx, "UPDATE users SET password_hash = $1, updated_at = now() WHERE id::text = $2", passHash, userID)
	if err != nil {
		return err
	}
	_, _ = r.db.Exec(ctx, "UPDATE sessions SET is_revoked = true WHERE user_id::text = $1 AND is_revoked = false", userID)
	_, _ = r.db.Exec(ctx, "DELETE FROM otp_requests WHERE phone = $1 AND purpose = 'PASSWORD_RESET'", email)
	return nil
}

type DBUser struct {
	ID                string
	Email             string
	Phone             string
	FullName          string
	Role              string
	Status            string
	EmailVerified     bool
	PhoneVerified     bool
	ProfilePictureURL string
	CreatedAt         time.Time
}

func (r *Repository) GetUserByID(ctx context.Context, userID string) (*DBUser, error) {
	if r.db == nil {
		return nil, nil
	}
	var id, fullName, role, status string
	var email, phone, profilePic *string
	var emailVerified, phoneVerified bool
	var createdAt time.Time

	err := r.db.QueryRow(ctx,
		`SELECT id::text, email, phone, full_name, role, status,
		        email_verified, phone_verified, profile_picture_url, created_at
		 FROM users WHERE id::text = $1`,
		userID,
	).Scan(&id, &email, &phone, &fullName, &role, &status, &emailVerified, &phoneVerified, &profilePic, &createdAt)
	if err != nil {
		return nil, err
	}
	emailStr, phoneStr, picStr := "", "", ""
	if email != nil { emailStr = *email }
	if phone != nil { phoneStr = *phone }
	if profilePic != nil { picStr = *profilePic }

	return &DBUser{
		ID:                id,
		Email:             emailStr,
		Phone:             phoneStr,
		FullName:          fullName,
		Role:              role,
		Status:            status,
		EmailVerified:     emailVerified,
		PhoneVerified:     phoneVerified,
		ProfilePictureURL: picStr,
		CreatedAt:         createdAt,
	}, nil
}

type DBSession struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Status    string    `json:"status"`
}

func (r *Repository) GetActiveSessions(ctx context.Context, userID string) ([]DBSession, error) {
	if r.db == nil {
		return []DBSession{}, nil
	}
	rows, err := r.db.Query(ctx,
		"SELECT id::text, created_at, expires_at FROM sessions WHERE user_id::text = $1 AND is_revoked = false AND expires_at > now() ORDER BY created_at DESC",
		userID,
	)
	if err != nil {
		return []DBSession{}, nil
	}
	defer rows.Close()

	var sessions []DBSession
	for rows.Next() {
		var s DBSession
		if err := rows.Scan(&s.ID, &s.CreatedAt, &s.ExpiresAt); err == nil {
			s.Status = "ACTIVE"
			sessions = append(sessions, s)
		}
	}
	if sessions == nil { sessions = []DBSession{} }
	return sessions, nil
}

func (r *Repository) RevokeSessionByID(ctx context.Context, sessionID, userID string) error {
	if r.db == nil {
		return nil
	}
	_, err := r.db.Exec(ctx, "UPDATE sessions SET is_revoked = true WHERE id::text = $1 AND user_id::text = $2", sessionID, userID)
	return err
}
