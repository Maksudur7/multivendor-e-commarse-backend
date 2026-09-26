package user

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yourusername/ecom-backend/pkg/response"
)

type Handler struct {
	db      *pgxpool.Pool
	service *Service
}

func NewHandler(db *pgxpool.Pool) *Handler {
	repo := NewRepository(db)
	service := NewService(repo)
	return &Handler{
		db:      db,
		service: service,
	}
}

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

	admin := router.Group("/admin/kyc", authMiddleware)
	admin.Put("/review/:userId", h.ReviewKYC)
}

func (h *Handler) GetProfile(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	if h.db == nil {
		return response.Error(c, fiber.StatusServiceUnavailable, "Database not connected", nil)
	}

	profile, err := h.service.GetProfile(c.Context(), userID)
	if err != nil || profile == nil {
		return response.NotFound(c, fmt.Sprintf("User not found (id=%s): %v", userID, err))
	}

	return response.Success(c, fiber.StatusOK, "User profile fetched from NeonDB", fiber.Map{
		"id":                  profile.ID,
		"email":               profile.Email,
		"phone":               profile.Phone,
		"full_name":           profile.FullName,
		"role":                profile.Role,
		"status":              profile.Status,
		"email_verified":      profile.EmailVerified,
		"phone_verified":      profile.PhoneVerified,
		"profile_picture_url": profile.ProfilePictureURL,
		"created_at":          profile.CreatedAt,
		"updated_at":          profile.UpdatedAt,
	})
}

type UpdateProfileReq struct {
	FullName          string `json:"full_name"`
	AvatarURL         string `json:"avatar_url"`
	Phone             string `json:"phone"`
	Email             string `json:"email"`
	DateOfBirth       string `json:"date_of_birth"`
	Gender            string `json:"gender"`
	PreferredLanguage string `json:"preferred_language"`
}

func (h *Handler) UpdateProfile(c *fiber.Ctx) error {
	var req UpdateProfileReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request payload: "+err.Error())
	}
	req.FullName = strings.TrimSpace(req.FullName)
	req.Email = strings.TrimSpace(req.Email)
	userID := c.Locals("user_id").(string)

	if h.db == nil {
		return response.Error(c, fiber.StatusServiceUnavailable, "Database not connected", nil)
	}

	if err := h.service.UpdateProfile(c.Context(), userID, req.FullName, req.AvatarURL, req.Phone, req.Email, req.DateOfBirth, req.Gender, req.PreferredLanguage); err != nil {
		if strings.Contains(err.Error(), "users_email_key") {
			return response.Conflict(c, "DUPLICATE_EMAIL", "This email address is already registered to another user account.")
		}
		if strings.Contains(err.Error(), "23505") || strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "users_phone_key") {
			return response.Conflict(c, "DUPLICATE_PHONE", "This phone number is already registered to another user account.")
		}
		return response.Error(c, fiber.StatusInternalServerError, fmt.Sprintf("Failed to update profile: %v", err), nil)
	}

	return response.Success(c, fiber.StatusOK, "Profile updated successfully in NeonDB", fiber.Map{
		"user_id":            userID,
		"full_name":          req.FullName,
		"avatar_url":         req.AvatarURL,
		"phone":              req.Phone,
		"email":              req.Email,
		"date_of_birth":      req.DateOfBirth,
		"gender":             req.Gender,
		"preferred_language": req.PreferredLanguage,
	})
}

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

	if h.db == nil {
		return response.Error(c, fiber.StatusServiceUnavailable, "Database not connected", nil)
	}

	if err := h.service.ChangePassword(c.Context(), userID, req.CurrentPassword, req.NewPassword); err != nil {
		if err.Error() == "user not found" {
			return response.NotFound(c, "User not found")
		}
		if err.Error() == "current password is incorrect" {
			return response.Error(c, fiber.StatusUnauthorized, "Current password is incorrect", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, fmt.Sprintf("Failed to update password: %v", err), nil)
	}

	return response.Success(c, fiber.StatusOK, "Password changed. Please login again.", fiber.Map{
		"status": "UPDATED",
	})
}

func (h *Handler) GetAddresses(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	addresses, err := h.service.GetAddresses(c.Context(), userID)
	if err != nil && h.db != nil {
		return response.Error(c, fiber.StatusInternalServerError, fmt.Sprintf("Failed to fetch addresses: %v", err), nil)
	}

	return response.Success(c, fiber.StatusOK, "Addresses fetched from NeonDB", fiber.Map{
		"user_id":   userID,
		"addresses": addresses,
		"count":     len(addresses),
	})
}

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

	recName, recPhone, line1, division, district := h.service.NormalizeAddressInput(
		req.RecipientName, req.FullName, req.RecipientPhone, req.Phone,
		req.AddressLine1, req.Address, req.Division, req.City, req.District,
	)

	if recName == "" || recPhone == "" || line1 == "" || division == "" {
		return response.ValidationError(c, map[string]string{
			"required": "recipient_name, recipient_phone, address_line1, division are mandatory",
		})
	}

	userID := c.Locals("user_id").(string)

	if h.db == nil {
		return response.Error(c, fiber.StatusServiceUnavailable, "Database not connected", nil)
	}

	addressID, err := h.service.CreateAddress(c.Context(), CreateAddressInput{
		UserID:         userID,
		RecipientName:  recName,
		RecipientPhone: recPhone,
		AddressLine1:   line1,
		AddressLine2:   req.AddressLine2,
		Division:       division,
		District:       district,
		Upazila:        req.Upazila,
		PostalCode:     req.PostalCode,
		Label:          req.Label,
		IsDefault:      req.IsDefault,
	})
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, fmt.Sprintf("Failed to save address: %v", err), nil)
	}

	return response.Created(c, "Address added successfully to NeonDB", fiber.Map{
		"address_id":     addressID,
		"id":             addressID,
		"recipient_name": recName,
		"address_line1":  line1,
		"division":       division,
		"district":       district,
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
	recName, recPhone, line1, _, _ := h.service.NormalizeAddressInput(
		req.RecipientName, req.FullName, req.RecipientPhone, req.Phone,
		req.AddressLine1, req.Address, req.Division, req.City, req.District,
	)

	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Address updated", fiber.Map{"address_id": addressID})
	}

	if err := h.service.UpdateAddress(c.Context(), addressID, userID, recName, recPhone, line1, req.Label); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to update address", nil)
	}

	return response.Success(c, fiber.StatusOK, "Address updated successfully", fiber.Map{"address_id": addressID})
}

func (h *Handler) SetDefaultAddress(c *fiber.Ctx) error {
	addressID := c.Params("id")
	userID := c.Locals("user_id").(string)

	if h.db == nil {
		return response.Error(c, fiber.StatusServiceUnavailable, "Database not connected", nil)
	}

	affected, err := h.service.SetDefaultAddress(c.Context(), addressID, userID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, fmt.Sprintf("Failed to set default: %v", err), nil)
	}
	if affected == 0 {
		return response.NotFound(c, "Address not found or does not belong to you")
	}

	return response.Success(c, fiber.StatusOK, "Default address updated in NeonDB", fiber.Map{
		"address_id": addressID,
		"is_default": true,
	})
}

func (h *Handler) DeleteAddress(c *fiber.Ctx) error {
	addressID := c.Params("id")
	userID := c.Locals("user_id").(string)

	if h.db == nil {
		return response.Error(c, fiber.StatusServiceUnavailable, "Database not connected", nil)
	}

	affected, err := h.service.DeleteAddress(c.Context(), addressID, userID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, fmt.Sprintf("Failed to delete address: %v", err), nil)
	}
	if affected == 0 {
		return response.NotFound(c, "Address not found or does not belong to you")
	}

	return response.Success(c, fiber.StatusOK, "Address deleted from NeonDB", fiber.Map{
		"deleted_id": addressID,
	})
}

type KYCSubmitReq struct {
	DocumentType   string `json:"document_type"`
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

	if !h.service.IsValidKYCDocumentType(req.DocumentType) {
		return response.ValidationError(c, map[string]string{
			"document_type": "Must be one of: NID, PASSPORT, DRIVING_LICENSE, TRADE_LICENSE",
		})
	}

	userID := c.Locals("user_id").(string)

	if h.db == nil {
		return response.Error(c, fiber.StatusServiceUnavailable, "Database not connected", nil)
	}

	kycID, err := h.service.SubmitKYC(c.Context(), userID, req.DocumentType, req.DocumentNumber,
		req.FrontImageURL, req.BackImageURL, req.SelfieImageURL)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, fmt.Sprintf("Failed to submit KYC: %v", err), nil)
	}

	return response.Success(c, fiber.StatusOK, "KYC documents submitted for verification in NeonDB", fiber.Map{
		"kyc_id":        kycID,
		"user_id":       userID,
		"status":        "PENDING",
		"document_type": req.DocumentType,
	})
}

type KYCReviewReq struct {
	Status          string `json:"status"`
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

	if h.db == nil {
		return response.Error(c, fiber.StatusServiceUnavailable, "Database not connected", nil)
	}

	affected, err := h.service.ReviewKYC(c.Context(), targetUserID, req.Status, req.RejectionReason)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, fmt.Sprintf("Failed to update KYC: %v", err), nil)
	}
	if affected == 0 {
		return response.NotFound(c, "No KYC submission found for this user")
	}

	return response.Success(c, fiber.StatusOK, "KYC reviewed and updated in NeonDB", fiber.Map{
		"target_user_id":   targetUserID,
		"new_status":       req.Status,
		"rejection_reason": req.RejectionReason,
	})
}
