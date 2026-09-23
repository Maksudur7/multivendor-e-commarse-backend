package user

import (
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/yourusername/ecom-backend/pkg/response"
)

// Handler handles user profile, address, and KYC operations.
type Handler struct {
	db *pgxpool.Pool
}

// NewHandler creates a new user HTTP handler.
func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{db: db}
}

// RegisterRoutes attaches user domain endpoints.
func (h *Handler) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	u := router.Group("/user", authMiddleware)
	u.Get("/profile", h.GetProfile)
	u.Put("/profile", h.UpdateProfile)
	u.Get("/addresses", h.GetAddresses)
	u.Post("/addresses", h.CreateAddress)
	u.Put("/addresses/:id", h.UpdateAddress)
	u.Put("/addresses/:id/default", h.SetDefaultAddress)
	u.Delete("/addresses/:id", h.DeleteAddress)
	u.Post("/kyc/submit", h.SubmitKYC)
	u.Post("/change-password", h.ChangePassword)

	// Admin route for KYC review
	admin := router.Group("/admin/kyc", authMiddleware)
	admin.Put("/review/:userId", h.ReviewKYC)
}

// ─────────────────────────────────────────────
// GET PROFILE — live from NeonDB
// ─────────────────────────────────────────────

func (h *Handler) GetProfile(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	ctx := c.Context()

	if h.db == nil {
		return response.Error(c, fiber.StatusServiceUnavailable, "Database not connected", nil)
	}

	var id, fullName, role, status string
	var email, phone, profilePic *string
	var emailVerified, phoneVerified bool
	var createdAt, updatedAt time.Time

	err := h.db.QueryRow(ctx,
		`SELECT id::text, email, phone, full_name, role, status,
		        email_verified, phone_verified, profile_picture_url,
		        created_at, updated_at
		 FROM users WHERE id::text = $1`,
		userID,
	).Scan(&id, &email, &phone, &fullName, &role, &status,
		&emailVerified, &phoneVerified, &profilePic,
		&createdAt, &updatedAt)

	if err != nil {
		return response.NotFound(c, fmt.Sprintf("User not found (id=%s): %v", userID, err))
	}

	emailStr := ""
	if email != nil {
		emailStr = *email
	}
	phoneStr := ""
	if phone != nil {
		phoneStr = *phone
	}
	picStr := ""
	if profilePic != nil {
		picStr = *profilePic
	}

	return response.Success(c, fiber.StatusOK, "User profile fetched from NeonDB", fiber.Map{
		"id":                  id,
		"email":               emailStr,
		"phone":               phoneStr,
		"full_name":           fullName,
		"role":                role,
		"status":              status,
		"email_verified":      emailVerified,
		"phone_verified":      phoneVerified,
		"profile_picture_url": picStr,
		"created_at":          createdAt,
		"updated_at":          updatedAt,
	})
}

// ─────────────────────────────────────────────
// UPDATE PROFILE — saves to NeonDB
// ─────────────────────────────────────────────

type UpdateProfileReq struct {
	FullName  string `json:"full_name"`
	AvatarURL string `json:"avatar_url"`
	Phone     string `json:"phone"`
}

func (h *Handler) UpdateProfile(c *fiber.Ctx) error {
	var req UpdateProfileReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request payload: "+err.Error())
	}
	req.FullName = strings.TrimSpace(req.FullName)
	userID := c.Locals("user_id").(string)
	ctx := c.Context()

	if h.db == nil {
		return response.Error(c, fiber.StatusServiceUnavailable, "Database not connected", nil)
	}

	// Build dynamic update query
	setClauses := []string{"updated_at = now()"}
	args := []interface{}{}
	argIdx := 1

	if req.FullName != "" {
		setClauses = append(setClauses, fmt.Sprintf("full_name = $%d", argIdx))
		args = append(args, req.FullName)
		argIdx++
	}
	if req.AvatarURL != "" {
		setClauses = append(setClauses, fmt.Sprintf("profile_picture_url = $%d", argIdx))
		args = append(args, req.AvatarURL)
		argIdx++
	}
	if req.Phone != "" {
		setClauses = append(setClauses, fmt.Sprintf("phone = $%d", argIdx))
		args = append(args, req.Phone)
		argIdx++
	}

	args = append(args, userID)
	query := fmt.Sprintf(
		"UPDATE users SET %s WHERE id::text = $%d",
		strings.Join(setClauses, ", "), argIdx,
	)

	_, err := h.db.Exec(ctx, query, args...)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, fmt.Sprintf("Failed to update profile: %v", err), nil)
	}

	return response.Success(c, fiber.StatusOK, "Profile updated successfully in NeonDB", fiber.Map{
		"user_id":   userID,
		"full_name": req.FullName,
		"avatar_url": req.AvatarURL,
		"phone":     req.Phone,
	})
}

// ─────────────────────────────────────────────
// CHANGE PASSWORD (authenticated)
// ─────────────────────────────────────────────

type ChangePasswordReq struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (h *Handler) ChangePassword(c *fiber.Ctx) error {
	var req ChangePasswordReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid payload")
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		return response.BadRequest(c, "current_password and new_password are required")
	}
	if len(req.NewPassword) < 6 {
		return response.ValidationError(c, map[string]string{"new_password": "Must be at least 6 characters"})
	}

	userID := c.Locals("user_id").(string)
	ctx := c.Context()

	if h.db == nil {
		return response.Error(c, fiber.StatusServiceUnavailable, "Database not connected", nil)
	}

	// Fetch stored hash
	var storedHash string
	err := h.db.QueryRow(ctx, "SELECT password_hash FROM users WHERE id::text = $1", userID).Scan(&storedHash)
	if err != nil {
		return response.NotFound(c, "User not found")
	}

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(req.CurrentPassword)); err != nil {
		return response.Error(c, fiber.StatusUnauthorized, "Current password is incorrect", nil)
	}

	// Hash new password
	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to secure new password", nil)
	}

	// Update in DB
	_, err = h.db.Exec(ctx,
		"UPDATE users SET password_hash = $1, updated_at = now() WHERE id::text = $2",
		string(newHash), userID,
	)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, fmt.Sprintf("Failed to update password: %v", err), nil)
	}

	// Revoke all sessions — force re-login
	_, _ = h.db.Exec(ctx,
		"UPDATE sessions SET is_revoked = true WHERE user_id::text = $1 AND is_revoked = false",
		userID,
	)

	return response.Success(c, fiber.StatusOK, "Password changed. Please login again.", fiber.Map{
		"status": "UPDATED",
	})
}

// ─────────────────────────────────────────────
// GET ADDRESSES — live from NeonDB
// ─────────────────────────────────────────────

func (h *Handler) GetAddresses(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	ctx := c.Context()

	type Address struct {
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

	var addresses []Address

	if h.db != nil {
		rows, err := h.db.Query(ctx,
			`SELECT id::text, recipient_name, recipient_phone, address_line1,
			        COALESCE(address_line2,''), division, district,
			        COALESCE(upazila,''), COALESCE(postal_code,''),
			        COALESCE(label,''), is_default, created_at
			 FROM user_addresses
			 WHERE user_id::text = $1
			 ORDER BY is_default DESC, created_at DESC`,
			userID,
		)
		if err != nil {
			return response.Error(c, fiber.StatusInternalServerError, fmt.Sprintf("Failed to fetch addresses: %v", err), nil)
		}
		defer rows.Close()
		for rows.Next() {
			var a Address
			if scanErr := rows.Scan(
				&a.ID, &a.RecipientName, &a.RecipientPhone, &a.AddressLine1,
				&a.AddressLine2, &a.Division, &a.District, &a.Upazila,
				&a.PostalCode, &a.Label, &a.IsDefault, &a.CreatedAt,
			); scanErr == nil {
				addresses = append(addresses, a)
			}
		}
	}

	if addresses == nil {
		addresses = []Address{}
	}

	return response.Success(c, fiber.StatusOK, "Addresses fetched from NeonDB", fiber.Map{
		"user_id":   userID,
		"addresses": addresses,
		"count":     len(addresses),
	})
}

// ─────────────────────────────────────────────
// CREATE ADDRESS — saves to NeonDB
// ─────────────────────────────────────────────

type AddressReq struct {
	RecipientName  string `json:"recipient_name"`
	FullName       string `json:"full_name"`
	RecipientPhone string `json:"recipient_phone"`
	Phone          string `json:"phone"`
	AddressLine1   string `json:"address_line1"`
	Address        string `json:"address"`
	AddressLine2   string `json:"address_line2"`
	Division       string `json:"division"`
	City           string `json:"city"`
	District       string `json:"district"`
	Upazila        string `json:"upazila"`
	PostalCode     string `json:"postal_code"`
	IsDefault      bool   `json:"is_default"`
	Label          string `json:"label"`
}

func (h *Handler) CreateAddress(c *fiber.Ctx) error {
	var req AddressReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid address input: "+err.Error())
	}
	if req.RecipientName == "" { req.RecipientName = req.FullName }
	if req.RecipientPhone == "" { req.RecipientPhone = req.Phone }
	if req.AddressLine1 == "" { req.AddressLine1 = req.Address }
	if req.Division == "" { req.Division = req.City }
	if req.District == "" { req.District = req.Division }

	if req.RecipientName == "" || req.RecipientPhone == "" || req.AddressLine1 == "" || req.Division == "" {
		return response.ValidationError(c, map[string]string{
			"required": "recipient_name, recipient_phone, address_line1, division are mandatory",
		})
	}

	userID := c.Locals("user_id").(string)
	ctx := c.Context()

	if h.db == nil {
		return response.Error(c, fiber.StatusServiceUnavailable, "Database not connected", nil)
	}

	// If set as default, unset existing defaults first
	if req.IsDefault {
		_, _ = h.db.Exec(ctx,
			"UPDATE user_addresses SET is_default = false WHERE user_id::text = $1",
			userID,
		)
	}

	var addressID string
	err := h.db.QueryRow(ctx,
		`INSERT INTO user_addresses
		 (user_id, recipient_name, recipient_phone, address_line1, address_line2,
		  division, district, upazila, postal_code, is_default, label)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING id::text`,
		userID, req.RecipientName, req.RecipientPhone, req.AddressLine1, req.AddressLine2,
		req.Division, req.District, req.Upazila, req.PostalCode, req.IsDefault, req.Label,
	).Scan(&addressID)

	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, fmt.Sprintf("Failed to save address: %v", err), nil)
	}

	return response.Created(c, "Address added successfully to NeonDB", fiber.Map{
		"address_id":     addressID,
		"id":             addressID,
		"recipient_name": req.RecipientName,
		"address_line1":  req.AddressLine1,
		"division":       req.Division,
		"district":       req.District,
		"is_default":     req.IsDefault,
	})
}

func (h *Handler) UpdateAddress(c *fiber.Ctx) error {
	addressID := c.Params("id")
	userID := c.Locals("user_id").(string)
	var req AddressReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid address payload: "+err.Error())
	}
	if req.RecipientName == "" { req.RecipientName = req.FullName }
	if req.RecipientPhone == "" { req.RecipientPhone = req.Phone }
	if req.AddressLine1 == "" { req.AddressLine1 = req.Address }

	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Address updated", fiber.Map{"address_id": addressID})
	}

	ctx := c.Context()
	_, err := h.db.Exec(ctx,
		`UPDATE user_addresses SET
		 recipient_name = COALESCE(NULLIF($1,''), recipient_name),
		 recipient_phone = COALESCE(NULLIF($2,''), recipient_phone),
		 address_line1 = COALESCE(NULLIF($3,''), address_line1),
		 label = COALESCE(NULLIF($4,''), label),
		 updated_at = now()
		 WHERE id::text = $5 AND user_id::text = $6`,
		req.RecipientName, req.RecipientPhone, req.AddressLine1, req.Label, addressID, userID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to update address", nil)
	}

	return response.Success(c, fiber.StatusOK, "Address updated successfully", fiber.Map{"address_id": addressID})
}

// ─────────────────────────────────────────────
// SET DEFAULT ADDRESS — updates NeonDB
// ─────────────────────────────────────────────

func (h *Handler) SetDefaultAddress(c *fiber.Ctx) error {
	addressID := c.Params("id")
	userID := c.Locals("user_id").(string)
	ctx := c.Context()

	if h.db == nil {
		return response.Error(c, fiber.StatusServiceUnavailable, "Database not connected", nil)
	}

	// Unset all defaults for this user
	_, _ = h.db.Exec(ctx,
		"UPDATE user_addresses SET is_default = false WHERE user_id::text = $1",
		userID,
	)

	// Set new default — only if it belongs to this user
	result, err := h.db.Exec(ctx,
		"UPDATE user_addresses SET is_default = true WHERE id::text = $1 AND user_id::text = $2",
		addressID, userID,
	)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, fmt.Sprintf("Failed to set default: %v", err), nil)
	}
	if result.RowsAffected() == 0 {
		return response.NotFound(c, "Address not found or does not belong to you")
	}

	return response.Success(c, fiber.StatusOK, "Default address updated in NeonDB", fiber.Map{
		"address_id": addressID,
		"is_default": true,
	})
}

// ─────────────────────────────────────────────
// DELETE ADDRESS — removes from NeonDB
// ─────────────────────────────────────────────

func (h *Handler) DeleteAddress(c *fiber.Ctx) error {
	addressID := c.Params("id")
	userID := c.Locals("user_id").(string)
	ctx := c.Context()

	if h.db == nil {
		return response.Error(c, fiber.StatusServiceUnavailable, "Database not connected", nil)
	}

	result, err := h.db.Exec(ctx,
		"DELETE FROM user_addresses WHERE id::text = $1 AND user_id::text = $2",
		addressID, userID,
	)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, fmt.Sprintf("Failed to delete address: %v", err), nil)
	}
	if result.RowsAffected() == 0 {
		return response.NotFound(c, "Address not found or does not belong to you")
	}

	return response.Success(c, fiber.StatusOK, "Address deleted from NeonDB", fiber.Map{
		"deleted_id": addressID,
	})
}

// ─────────────────────────────────────────────
// SUBMIT KYC — saves to NeonDB
// ─────────────────────────────────────────────

type KYCSubmitReq struct {
	DocumentType   string `json:"document_type"`   // NID, PASSPORT, DRIVING_LICENSE, TRADE_LICENSE
	DocumentNumber string `json:"document_number"`
	FrontImageURL  string `json:"front_image_url"`
	BackImageURL   string `json:"back_image_url"`
	SelfieImageURL string `json:"selfie_image_url"`
}

func (h *Handler) SubmitKYC(c *fiber.Ctx) error {
	var req KYCSubmitReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid KYC request payload: "+err.Error())
	}
	if req.DocumentType == "" || req.DocumentNumber == "" || req.FrontImageURL == "" {
		return response.ValidationError(c, map[string]string{
			"kyc": "document_type, document_number, and front_image_url are required",
		})
	}

	validTypes := map[string]bool{
		"NID": true, "PASSPORT": true, "DRIVING_LICENSE": true, "TRADE_LICENSE": true,
	}
	if !validTypes[req.DocumentType] {
		return response.ValidationError(c, map[string]string{
			"document_type": "Must be one of: NID, PASSPORT, DRIVING_LICENSE, TRADE_LICENSE",
		})
	}

	userID := c.Locals("user_id").(string)
	ctx := c.Context()

	if h.db == nil {
		return response.Error(c, fiber.StatusServiceUnavailable, "Database not connected", nil)
	}

	kycID := uuid.New().String()
	_, err := h.db.Exec(ctx,
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
		kycID, userID, req.DocumentType, req.DocumentNumber,
		req.FrontImageURL, req.BackImageURL, req.SelfieImageURL,
	)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, fmt.Sprintf("Failed to submit KYC: %v", err), nil)
	}

	return response.Success(c, fiber.StatusOK, "KYC documents submitted for verification in NeonDB", fiber.Map{
		"kyc_id":       kycID,
		"user_id":      userID,
		"status":       "PENDING",
		"document_type": req.DocumentType,
	})
}

// ─────────────────────────────────────────────
// REVIEW KYC (Admin) — updates NeonDB
// ─────────────────────────────────────────────

type KYCReviewReq struct {
	Status          string `json:"status"` // VERIFIED, REJECTED
	RejectionReason string `json:"rejection_reason"`
}

func (h *Handler) ReviewKYC(c *fiber.Ctx) error {
	targetUserID := c.Params("userId")
	var req KYCReviewReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid payload: "+err.Error())
	}
	if req.Status != "VERIFIED" && req.Status != "REJECTED" {
		return response.BadRequest(c, "Status must be VERIFIED or REJECTED")
	}

	ctx := c.Context()

	if h.db == nil {
		return response.Error(c, fiber.StatusServiceUnavailable, "Database not connected", nil)
	}

	result, err := h.db.Exec(ctx,
		`UPDATE kyc_verifications
		 SET status = $1, rejection_reason = $2, reviewed_at = now(), updated_at = now()
		 WHERE user_id::text = $3`,
		req.Status, req.RejectionReason, targetUserID,
	)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, fmt.Sprintf("Failed to update KYC: %v", err), nil)
	}
	if result.RowsAffected() == 0 {
		return response.NotFound(c, "No KYC submission found for this user")
	}

	// If verified, update user's kyc_status in users table too
	if req.Status == "VERIFIED" {
		_, _ = h.db.Exec(ctx,
			"UPDATE users SET kyc_status = 'VERIFIED', updated_at = now() WHERE id::text = $1",
			targetUserID,
		)
	}

	return response.Success(c, fiber.StatusOK, "KYC reviewed and updated in NeonDB", fiber.Map{
		"target_user_id":   targetUserID,
		"new_status":       req.Status,
		"rejection_reason": req.RejectionReason,
	})
}
