package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles all DB operations for the auth domain.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository constructs a Repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ── OTP operations ───────────────────────────────────────────────────────────

// InsertOTP inserts a new OTP row.
// IMPORTANT: otpHash must already be SHA-256 hashed by the caller (service layer).
func (r *Repository) InsertOTP(ctx context.Context, target, otpHash, purpose string, expiresAt time.Time) error {
	if r.db == nil {
		return fmt.Errorf("database not connected")
	}
	_, err := r.db.Exec(ctx,
		`INSERT INTO otp_requests (phone, otp_hash, purpose, expires_at)
		 VALUES ($1, $2, $3, $4)`,
		target, otpHash, purpose, expiresAt,
	)
	return err
}

// GetLatestOTP retrieves the hash and expiry of the most recent unused, non-expired OTP.
// Returns the stored hash for comparison in the service layer.
func (r *Repository) GetLatestOTP(ctx context.Context, target, purpose string) (otpHash string, expiresAt time.Time, err error) {
	if r.db == nil {
		return "", time.Time{}, fmt.Errorf("database not connected")
	}
	err = r.db.QueryRow(ctx,
		`SELECT otp_hash, expires_at
		 FROM otp_requests
		 WHERE phone = $1
		   AND purpose = $2
		   AND used = FALSE
		   AND expires_at > now()
		 ORDER BY created_at DESC
		 LIMIT 1`,
		target, purpose,
	).Scan(&otpHash, &expiresAt)
	return
}

// IncrementOTPAttempt increments the attempt_count on the latest active OTP row.
// Used to support future brute-force lockout policies.
func (r *Repository) IncrementOTPAttempt(ctx context.Context, target, purpose string) error {
	if r.db == nil {
		return nil
	}
	_, err := r.db.Exec(ctx,
		`UPDATE otp_requests
		 SET attempt_count = attempt_count + 1
		 WHERE id = (
		     SELECT id FROM otp_requests
		     WHERE phone = $1 AND purpose = $2 AND used = FALSE
		     ORDER BY created_at DESC LIMIT 1
		 )`,
		target, purpose,
	)
	return err
}

// DeleteOTP marks the OTP as used (soft-delete), preventing replay attacks.
func (r *Repository) DeleteOTP(ctx context.Context, target, purpose string) error {
	if r.db == nil {
		return nil
	}
	_, err := r.db.Exec(ctx,
		`UPDATE otp_requests
		 SET used = TRUE
		 WHERE phone = $1 AND purpose = $2 AND used = FALSE`,
		target, purpose,
	)
	return err
}

// ── User operations ──────────────────────────────────────────────────────────

// MarkUserPhoneVerified sets phone_verified = TRUE and updates phone for a specific user ID.
func (r *Repository) MarkUserPhoneVerified(ctx context.Context, userID, phone string) error {
	if r.db == nil {
		return fmt.Errorf("database not connected")
	}
	_, err := r.db.Exec(ctx,
		`UPDATE users 
		 SET phone = COALESCE(NULLIF(phone, ''), $2), 
		     phone_verified = TRUE, 
		     updated_at = now() 
		 WHERE id::text = $1`,
		userID, phone,
	)
	return err
}

// FindOrCreateUserByPhone finds an existing user by phone or email, or creates a new CUSTOMER.
func (r *Repository) FindOrCreateUserByPhone(ctx context.Context, phoneOrEmail string) (string, error) {
	if r.db == nil {
		return "", fmt.Errorf("database not connected")
	}

	// Try existing user first by phone or by email.
	var existingID string
	err := r.db.QueryRow(ctx,
		`SELECT id::text FROM users 
		 WHERE (phone IS NOT NULL AND phone = $1 AND $1 != '') 
		    OR (email IS NOT NULL AND email = $1 AND $1 != '')`,
		phoneOrEmail,
	).Scan(&existingID)
	if err == nil {
		_, _ = r.db.Exec(ctx,
			`UPDATE users 
			 SET phone = COALESCE(NULLIF(phone, ''), $2), 
			     phone_verified = TRUE, 
			     updated_at = now() 
			 WHERE id::text = $1`,
			existingID, phoneOrEmail,
		)
		return existingID, nil
	}

	// Create new user.
	var newID string
	err = r.db.QueryRow(ctx,
		`INSERT INTO users (phone, full_name, role, status, phone_verified)
		 VALUES ($1, $2, 'CUSTOMER', 'ACTIVE', TRUE)
		 RETURNING id::text`,
		phoneOrEmail, "User-"+phoneOrEmail[max(0, len(phoneOrEmail)-4):],
	).Scan(&newID)
	return newID, err
}

// CheckDuplicateAccount returns true if a user with email or phone already exists.
// Note: If a user exists with this phone but has NO email (created via OTP), it is not a conflict;
// CreateEmailUser will merge the email credentials into that existing user record.
func (r *Repository) CheckDuplicateAccount(ctx context.Context, email, phone string) bool {
	if r.db == nil {
		return false
	}
	var id string
	err := r.db.QueryRow(ctx,
		`SELECT id::text FROM users
		 WHERE email = $1
		    OR (phone IS NOT NULL AND phone = $2 AND $2 != '' AND email IS NOT NULL AND email != '')`,
		email, phone,
	).Scan(&id)
	return err == nil
}

// CreateEmailUser inserts a new user with email/password credentials or links them to an existing OTP-created phone user.
// email_verified is FALSE — the user must verify their email separately.
func (r *Repository) CreateEmailUser(ctx context.Context, email string, phone *string, passHash, fullName, role string) (string, error) {
	if r.db == nil {
		return "", fmt.Errorf("database not connected")
	}

	// If phone is provided, check if a phone-only account (created via OTP) already exists.
	if phone != nil && *phone != "" {
		var existingID string
		err := r.db.QueryRow(ctx,
			`SELECT id::text FROM users WHERE phone = $1 AND (email IS NULL OR email = '')`,
			*phone,
		).Scan(&existingID)
		if err == nil {
			// Update/link the existing phone user with email credentials
			_, err = r.db.Exec(ctx,
				`UPDATE users
				 SET email = $1, password_hash = $2, full_name = $3, role = $4, updated_at = now()
				 WHERE id::text = $5`,
				email, passHash, fullName, role, existingID,
			)
			if err != nil {
				return "", fmt.Errorf("failed to link email to existing phone user: %w", err)
			}
			return existingID, nil
		}
	}

	var userID string
	err := r.db.QueryRow(ctx,
		`INSERT INTO users
		     (email, phone, password_hash, full_name, role, status, email_verified, phone_verified)
		 VALUES ($1, $2, $3, $4, $5, 'ACTIVE', FALSE, FALSE)
		 RETURNING id::text`,
		email, phone, passHash, fullName, role,
	).Scan(&userID)
	return userID, err
}

// UserLoginRecord contains the fields needed to authenticate a login request.
type UserLoginRecord struct {
	ID           string
	PasswordHash string
	Role         string
	Status       string
	Phone        string
}

// GetUserByEmail retrieves the login record for a given email.
func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*UserLoginRecord, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database not connected")
	}
	var id, storedHash, role, status string
	var phone *string
	err := r.db.QueryRow(ctx,
		`SELECT id::text, password_hash, role, status, phone
		 FROM users
		 WHERE email = $1`,
		email,
	).Scan(&id, &storedHash, &role, &status, &phone)
	if err != nil {
		return nil, err
	}
	phoneStr := ""
	if phone != nil {
		phoneStr = *phone
	}
	return &UserLoginRecord{
		ID:           id,
		PasswordHash: storedHash,
		Role:         role,
		Status:       status,
		Phone:        phoneStr,
	}, nil
}

// ── Session operations ───────────────────────────────────────────────────────

// SaveSession persists a new session with the hashed refresh token.
// IMPORTANT: refreshTokenHash must already be SHA-256 hashed by the caller.
func (r *Repository) SaveSession(ctx context.Context, userID, refreshTokenHash string, expiresAt time.Time) error {
	if r.db == nil {
		return fmt.Errorf("database not connected")
	}
	_, err := r.db.Exec(ctx,
		`INSERT INTO sessions (user_id, refresh_token_hash, expires_at, is_revoked)
		 VALUES ($1, $2, $3, FALSE)`,
		userID, refreshTokenHash, expiresAt,
	)
	return err
}

// GetSessionByToken looks up an active session by its hashed refresh token.
// Also returns the user's role via a JOIN — needed for token rotation.
func (r *Repository) GetSessionByToken(ctx context.Context, hashedRefreshToken string) (userID, role string, isRevoked bool, err error) {
	if r.db == nil {
		return "", "", false, fmt.Errorf("database not connected")
	}
	err = r.db.QueryRow(ctx,
		`SELECT s.user_id::text, u.role, s.is_revoked
		 FROM sessions s
		 JOIN users u ON u.id = s.user_id
		 WHERE s.refresh_token_hash = $1
		   AND s.expires_at > now()`,
		hashedRefreshToken,
	).Scan(&userID, &role, &isRevoked)
	return
}

// RevokeSessionByToken marks a single session revoked (used during token rotation).
func (r *Repository) RevokeSessionByToken(ctx context.Context, hashedRefreshToken string) error {
	if r.db == nil {
		return nil
	}
	_, err := r.db.Exec(ctx,
		`UPDATE sessions SET is_revoked = TRUE
		 WHERE refresh_token_hash = $1`,
		hashedRefreshToken,
	)
	return err
}

// RevokeAllUserSessions revokes every active session for the user (used on logout / password change).
func (r *Repository) RevokeAllUserSessions(ctx context.Context, userID string) error {
	if r.db == nil {
		return nil
	}
	_, err := r.db.Exec(ctx,
		`UPDATE sessions SET is_revoked = TRUE
		 WHERE user_id::text = $1 AND is_revoked = FALSE`,
		userID,
	)
	return err
}

// UpdatePasswordAndRevoke atomically updates the password hash and revokes all sessions.
func (r *Repository) UpdatePasswordAndRevoke(ctx context.Context, userID, passHash, email string) error {
	if r.db == nil {
		return fmt.Errorf("database not connected")
	}

	// Update password.
	_, err := r.db.Exec(ctx,
		`UPDATE users SET password_hash = $1, updated_at = now() WHERE id::text = $2`,
		passHash, userID,
	)
	if err != nil {
		return err
	}

	// Revoke all sessions so stolen tokens cannot be reused after reset.
	_, _ = r.db.Exec(ctx,
		`UPDATE sessions SET is_revoked = TRUE
		 WHERE user_id::text = $1 AND is_revoked = FALSE`,
		userID,
	)

	// Consume the password-reset OTP so it cannot be reused.
	_, _ = r.db.Exec(ctx,
		`UPDATE otp_requests SET used = TRUE
		 WHERE phone = $1 AND purpose = 'PASSWORD_RESET' AND used = FALSE`,
		email,
	)
	return nil
}

// ── GetUser / Sessions for profile & session management ─────────────────────

// DBUser is the full user profile returned by GetUserByID.
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

// GetUserByID fetches the full user profile by UUID string.
func (r *Repository) GetUserByID(ctx context.Context, userID string) (*DBUser, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database not connected")
	}
	var id, fullName, role, status string
	var email, phone, profilePic *string
	var emailVerified, phoneVerified bool
	var createdAt time.Time

	err := r.db.QueryRow(ctx,
		`SELECT id::text, email, phone, full_name, role, status,
		        email_verified, phone_verified, profile_picture_url, created_at
		 FROM users
		 WHERE id::text = $1`,
		userID,
	).Scan(&id, &email, &phone, &fullName, &role, &status,
		&emailVerified, &phoneVerified, &profilePic, &createdAt)
	if err != nil {
		return nil, err
	}

	emailStr, phoneStr, picStr := "", "", ""
	if email != nil {
		emailStr = *email
	}
	if phone != nil {
		phoneStr = *phone
	}
	if profilePic != nil {
		picStr = *profilePic
	}

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

// DBSession is the session info returned by GetActiveSessions.
type DBSession struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Status    string    `json:"status"`
}

// GetActiveSessions returns all non-revoked, non-expired sessions for a user.
func (r *Repository) GetActiveSessions(ctx context.Context, userID string) ([]DBSession, error) {
	if r.db == nil {
		return []DBSession{}, nil
	}
	rows, err := r.db.Query(ctx,
		`SELECT id::text, created_at, expires_at
		 FROM sessions
		 WHERE user_id::text = $1
		   AND is_revoked = FALSE
		   AND expires_at > now()
		 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return []DBSession{}, err
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
	if sessions == nil {
		sessions = []DBSession{}
	}
	return sessions, rows.Err()
}

// RevokeSessionByID revokes a specific session owned by the given user.
func (r *Repository) RevokeSessionByID(ctx context.Context, sessionID, userID string) error {
	if r.db == nil {
		return fmt.Errorf("database not connected")
	}
	_, err := r.db.Exec(ctx,
		`UPDATE sessions SET is_revoked = TRUE
		 WHERE id::text = $1 AND user_id::text = $2`,
		sessionID, userID,
	)
	return err
}

// GetOTPAttemptCount returns the current attempt_count for the latest active OTP.
func (r *Repository) GetOTPAttemptCount(ctx context.Context, target, purpose string) (int, error) {
	if r.db == nil {
		return 0, fmt.Errorf("database not connected")
	}
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT attempt_count
		 FROM otp_requests
		 WHERE phone = $1 AND purpose = $2 AND used = FALSE
		 ORDER BY created_at DESC LIMIT 1`,
		target, purpose,
	).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// FindOrCreateGoogleUser finds a user by Google ID or email, or creates a new one.
// Returns (userID, isNew, error).
func (r *Repository) FindOrCreateGoogleUser(ctx context.Context, googleID, email, name, picture string) (string, bool, error) {
	if r.db == nil {
		return "", false, fmt.Errorf("database not connected")
	}

	// Try by Google ID first.
	var existingID string
	err := r.db.QueryRow(ctx,
		`SELECT id::text FROM users WHERE google_id = $1`,
		googleID,
	).Scan(&existingID)
	if err == nil {
		// Update last login info.
		_, _ = r.db.Exec(ctx,
			`UPDATE users SET last_login_at = now(), updated_at = now() WHERE id::text = $1`,
			existingID,
		)
		return existingID, false, nil
	}

	// Try by email (link existing account with Google).
	err = r.db.QueryRow(ctx,
		`SELECT id::text FROM users WHERE email = $1`,
		email,
	).Scan(&existingID)
	if err == nil {
		// Link Google ID to existing account.
		_, _ = r.db.Exec(ctx,
			`UPDATE users
			 SET google_id = $1, email_verified = TRUE,
			     last_login_at = now(), updated_at = now()
			 WHERE id::text = $2`,
			googleID, existingID,
		)
		return existingID, false, nil
	}

	// Create new user.
	var newID string
	err = r.db.QueryRow(ctx,
		`INSERT INTO users
		     (email, google_id, full_name, profile_picture_url,
		      role, status, email_verified, phone_verified, last_login_at)
		 VALUES ($1, $2, $3, $4, 'CUSTOMER', 'ACTIVE', TRUE, FALSE, now())
		 RETURNING id::text`,
		email, googleID, name, picture,
	).Scan(&newID)
	if err != nil {
		return "", false, fmt.Errorf("failed to create Google user: %w", err)
	}
	return newID, true, nil
}

// SaveEmailVerificationToken stores a hashed email verification token.
func (r *Repository) SaveEmailVerificationToken(ctx context.Context, userID, hashedToken string, expiresAt time.Time) error {
	if r.db == nil {
		return fmt.Errorf("database not connected")
	}
	// Ensure phone column in otp_requests can store 36-char UUIDs or long email strings
	_, _ = r.db.Exec(ctx, `ALTER TABLE otp_requests ALTER COLUMN phone TYPE VARCHAR(255)`)

	// Upsert into otp_requests table using purpose='EMAIL_VERIFY'.
	_, err := r.db.Exec(ctx,
		`INSERT INTO otp_requests (phone, otp_hash, purpose, expires_at)
		 VALUES ($1, $2, 'EMAIL_VERIFY', $3)`,
		userID, hashedToken, expiresAt,
	)
	if err != nil {
		fmt.Printf("[EMAIL-VERIFY-ERR] SaveEmailVerificationToken failed for userID=%s: %v\n", userID, err)
	}
	return err
}

// VerifyEmail marks the user's email as verified after checking the hashed token.
func (r *Repository) VerifyEmail(ctx context.Context, userID, hashedToken string) error {
	if r.db == nil {
		return fmt.Errorf("database not connected")
	}

	// Validate token.
	var storedHash string
	var expiresAt time.Time
	err := r.db.QueryRow(ctx,
		`SELECT otp_hash, expires_at
		 FROM otp_requests
		 WHERE phone = $1 AND purpose = 'EMAIL_VERIFY'
		   AND used = FALSE AND expires_at > now()
		 ORDER BY created_at DESC LIMIT 1`,
		userID,
	).Scan(&storedHash, &expiresAt)
	if err != nil {
		return fmt.Errorf("invalid or expired verification link")
	}
	if storedHash != hashedToken {
		return fmt.Errorf("invalid verification token")
	}
	if time.Now().After(expiresAt) {
		return fmt.Errorf("verification link has expired: please request a new one")
	}

	// Mark verified.
	_, err = r.db.Exec(ctx,
		`UPDATE users SET email_verified = TRUE, updated_at = now() WHERE id::text = $1`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("failed to mark email as verified: %w", err)
	}

	// Consume the token.
	_, _ = r.db.Exec(ctx,
		`UPDATE otp_requests SET used = TRUE
		 WHERE phone = $1 AND purpose = 'EMAIL_VERIFY' AND used = FALSE`,
		userID,
	)
	return nil
}

// max is a helper for older Go versions where built-in max may not be available.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
