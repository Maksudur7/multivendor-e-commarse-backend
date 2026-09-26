package user

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

type UserProfile struct {
	ID                string    `json:"id"`
	Email             string    `json:"email"`
	Phone             string    `json:"phone"`
	FullName          string    `json:"full_name"`
	Role              string    `json:"role"`
	Status            string    `json:"status"`
	EmailVerified     bool      `json:"email_verified"`
	PhoneVerified     bool      `json:"phone_verified"`
	ProfilePictureURL string    `json:"profile_picture_url"`
	DateOfBirth       string    `json:"date_of_birth,omitempty"`
	Gender            string    `json:"gender,omitempty"`
	PreferredLanguage string    `json:"preferred_language,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func (r *Repository) GetProfileByID(ctx context.Context, userID string) (*UserProfile, error) {
	if r.db == nil {
		return nil, nil
	}
	var id, fullName, role, status string
	var email, phone, profilePic, dob, gender, prefLang *string
	var emailVerified, phoneVerified bool
	var createdAt, updatedAt time.Time

	err := r.db.QueryRow(ctx,
		`SELECT id::text, email, phone, full_name, role, status,
		        email_verified, phone_verified, profile_picture_url,
		        date_of_birth::text, gender, preferred_language,
		        created_at, updated_at
		 FROM users WHERE id::text = $1`,
		userID,
	).Scan(&id, &email, &phone, &fullName, &role, &status,
		&emailVerified, &phoneVerified, &profilePic,
		&dob, &gender, &prefLang,
		&createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}

	emailStr, phoneStr, picStr, dobStr, genderStr, langStr := "", "", "", "", "", ""
	if email != nil { emailStr = *email }
	if phone != nil { phoneStr = *phone }
	if profilePic != nil { picStr = *profilePic }
	if dob != nil { dobStr = *dob }
	if gender != nil { genderStr = *gender }
	if prefLang != nil { langStr = *prefLang }

	return &UserProfile{
		ID:                id,
		Email:             emailStr,
		Phone:             phoneStr,
		FullName:          fullName,
		Role:              role,
		Status:            status,
		EmailVerified:     emailVerified,
		PhoneVerified:     phoneVerified,
		ProfilePictureURL: picStr,
		DateOfBirth:       dobStr,
		Gender:            genderStr,
		PreferredLanguage: langStr,
		CreatedAt:         createdAt,
		UpdatedAt:         updatedAt,
	}, nil
}

func (r *Repository) UpdateProfile(ctx context.Context, userID, fullName, avatarURL, phone, email, dateOfBirth, gender, preferredLanguage string) error {
	if r.db == nil {
		return nil
	}
	setClauses := []string{"updated_at = now()"}
	args := []interface{}{}
	argIdx := 1

	if fullName != "" {
		setClauses = append(setClauses, fmt.Sprintf("full_name = $%d", argIdx))
		args = append(args, fullName)
		argIdx++
	}
	if avatarURL != "" {
		setClauses = append(setClauses, fmt.Sprintf("profile_picture_url = $%d", argIdx))
		args = append(args, avatarURL)
		argIdx++
	}
	if phone != "" {
		setClauses = append(setClauses, fmt.Sprintf("phone = $%d", argIdx))
		args = append(args, phone)
		argIdx++
	}
	if email != "" {
		setClauses = append(setClauses, fmt.Sprintf("email = $%d", argIdx))
		args = append(args, email)
		argIdx++
	}
	if dateOfBirth != "" {
		setClauses = append(setClauses, fmt.Sprintf("date_of_birth = $%d::date", argIdx))
		args = append(args, dateOfBirth)
		argIdx++
	}
	if gender != "" {
		setClauses = append(setClauses, fmt.Sprintf("gender = $%d", argIdx))
		args = append(args, gender)
		argIdx++
	}
	if preferredLanguage != "" {
		setClauses = append(setClauses, fmt.Sprintf("preferred_language = $%d", argIdx))
		args = append(args, preferredLanguage)
		argIdx++
	}

	args = append(args, userID)
	query := fmt.Sprintf("UPDATE users SET %s WHERE id::text = $%d", strings.Join(setClauses, ", "), argIdx)
	_, err := r.db.Exec(ctx, query, args...)
	return err
}

func (r *Repository) GetUserPasswordHash(ctx context.Context, userID string) (string, error) {
	if r.db == nil {
		return "", nil
	}
	var hash string
	err := r.db.QueryRow(ctx, "SELECT password_hash FROM users WHERE id::text = $1", userID).Scan(&hash)
	return hash, err
}

func (r *Repository) UpdatePasswordAndRevokeSessions(ctx context.Context, userID, newHash string) error {
	if r.db == nil {
		return nil
	}
	_, err := r.db.Exec(ctx, "UPDATE users SET password_hash = $1, updated_at = now() WHERE id::text = $2", newHash, userID)
	if err != nil {
		return err
	}
	_, _ = r.db.Exec(ctx, "UPDATE sessions SET is_revoked = true WHERE user_id::text = $1 AND is_revoked = false", userID)
	return nil
}

type UserAddress struct {
	ID             string    `json:"id"`
	RecipientName  string    `json:"recipient_name"`
	RecipientPhone string    `json:"recipient_phone"`
	AddressLine1   string    `json:"address_line1"`
	AddressLine2   string    `json:"address_line2"`
	Division       string    `json:"division"`
	District       string    `json:"district"`
	Upazila        string    `json:"upazila"`
	PostalCode     string    `json:"postal_code"`
	Label          string    `json:"label"`
	IsDefault      bool      `json:"is_default"`
	CreatedAt      time.Time `json:"created_at"`
}

func (r *Repository) GetAddresses(ctx context.Context, userID string) ([]UserAddress, error) {
	if r.db == nil {
		return []UserAddress{}, nil
	}
	rows, err := r.db.Query(ctx,
		`SELECT id::text, recipient_name, recipient_phone, address_line1,
		        COALESCE(address_line2,''), division, district,
		        COALESCE(upazila,''), COALESCE(postal_code,''),
		        COALESCE(label,''), is_default, created_at
		 FROM user_addresses WHERE user_id::text = $1 ORDER BY is_default DESC, created_at DESC`,
		userID,
	)
	if err != nil {
		return []UserAddress{}, err
	}
	defer rows.Close()

	var addresses []UserAddress
	for rows.Next() {
		var a UserAddress
		if scanErr := rows.Scan(
			&a.ID, &a.RecipientName, &a.RecipientPhone, &a.AddressLine1,
			&a.AddressLine2, &a.Division, &a.District, &a.Upazila,
			&a.PostalCode, &a.Label, &a.IsDefault, &a.CreatedAt,
		); scanErr == nil {
			addresses = append(addresses, a)
		}
	}
	if addresses == nil { addresses = []UserAddress{} }
	return addresses, nil
}

func (r *Repository) CreateAddress(ctx context.Context, userID, recipientName, recipientPhone, line1, line2, division, district, upazila, postalCode, label string, isDefault bool) (string, error) {
	if r.db == nil {
		return "", nil
	}
	if isDefault {
		_, _ = r.db.Exec(ctx, "UPDATE user_addresses SET is_default = false WHERE user_id::text = $1", userID)
	}
	var addressID string
	err := r.db.QueryRow(ctx,
		`INSERT INTO user_addresses
		 (user_id, recipient_name, recipient_phone, address_line1, address_line2,
		  division, district, upazila, postal_code, is_default, label)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING id::text`,
		userID, recipientName, recipientPhone, line1, line2, division, district, upazila, postalCode, isDefault, label,
	).Scan(&addressID)
	return addressID, err
}

func (r *Repository) UpdateAddress(ctx context.Context, addressID, userID, recipientName, recipientPhone, line1, label string) error {
	if r.db == nil {
		return nil
	}
	_, err := r.db.Exec(ctx,
		`UPDATE user_addresses SET
		 recipient_name = COALESCE(NULLIF($1,''), recipient_name),
		 recipient_phone = COALESCE(NULLIF($2,''), recipient_phone),
		 address_line1 = COALESCE(NULLIF($3,''), address_line1),
		 label = COALESCE(NULLIF($4,''), label),
		 updated_at = now()
		 WHERE id::text = $5 AND user_id::text = $6`,
		recipientName, recipientPhone, line1, label, addressID, userID)
	return err
}

func (r *Repository) SetDefaultAddress(ctx context.Context, addressID, userID string) (int64, error) {
	if r.db == nil {
		return 0, nil
	}
	_, _ = r.db.Exec(ctx, "UPDATE user_addresses SET is_default = false WHERE user_id::text = $1", userID)
	res, err := r.db.Exec(ctx, "UPDATE user_addresses SET is_default = true WHERE id::text = $1 AND user_id::text = $2", addressID, userID)
	if err != nil { return 0, err }
	return res.RowsAffected(), nil
}

func (r *Repository) DeleteAddress(ctx context.Context, addressID, userID string) (int64, error) {
	if r.db == nil {
		return 0, nil
	}
	res, err := r.db.Exec(ctx, "DELETE FROM user_addresses WHERE id::text = $1 AND user_id::text = $2", addressID, userID)
	if err != nil { return 0, err }
	return res.RowsAffected(), nil
}

func (r *Repository) SaveKYC(ctx context.Context, kycID, userID, docType, docNum, frontURL, backURL, selfieURL string) error {
	if r.db == nil {
		return nil
	}
	_, err := r.db.Exec(ctx,
		`INSERT INTO kyc_verifications
		 (id, user_id, document_type, document_number, front_image_url, back_image_url, selfie_image_url, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, 'PENDING')
		 ON CONFLICT (user_id) DO UPDATE SET
		   document_type = EXCLUDED.document_type,
		   document_number = EXCLUDED.document_number,
		   front_image_url = EXCLUDED.front_image_url,
		   back_image_url = EXCLUDED.back_image_url,
		   selfie_image_url = EXCLUDED.selfie_image_url,
		   status = 'PENDING',
		   updated_at = now()`,
		kycID, userID, docType, docNum, frontURL, backURL, selfieURL,
	)
	return err
}

func (r *Repository) ReviewKYC(ctx context.Context, targetUserID, status, reason string) (int64, error) {
	if r.db == nil {
		return 0, nil
	}
	res, err := r.db.Exec(ctx,
		`UPDATE kyc_verifications
		 SET status = $1, rejection_reason = $2, reviewed_at = now(), updated_at = now()
		 WHERE user_id::text = $3`,
		status, reason, targetUserID,
	)
	if err != nil { return 0, err }
	if status == "VERIFIED" && res.RowsAffected() > 0 {
		_, _ = r.db.Exec(ctx, "UPDATE users SET kyc_status = 'VERIFIED', updated_at = now() WHERE id::text = $1", targetUserID)
	}
	return res.RowsAffected(), nil
}
