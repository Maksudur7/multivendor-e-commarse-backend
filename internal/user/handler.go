package user

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
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
	u.Put("/addresses/:id/default", h.SetDefaultAddress)
	u.Delete("/addresses/:id", h.DeleteAddress)
	u.Post("/kyc/submit", h.SubmitKYC)

	// Admin route for KYC review
	admin := router.Group("/admin/kyc", authMiddleware)
	admin.Put("/review/:userId", h.ReviewKYC)
}

func (h *Handler) GetProfile(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	// In production sqlc generated methods will be called here
	return response.Success(c, fiber.StatusOK, "User profile fetched successfully", fiber.Map{
		"user_id": userID,
		"email":   c.Locals("email"),
		"role":    c.Locals("role"),
	})
}

type UpdateProfileReq struct {
	FullName  string `json:"full_name"`
	AvatarURL string `json:"avatar_url"`
}

func (h *Handler) UpdateProfile(c *fiber.Ctx) error {
	var req UpdateProfileReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request payload: "+err.Error())
	}
	userID := c.Locals("user_id").(string)
	return response.Success(c, fiber.StatusOK, "Profile updated successfully", fiber.Map{
		"user_id":   userID,
		"full_name": req.FullName,
		"avatar_url": req.AvatarURL,
	})
}

func (h *Handler) GetAddresses(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	return response.Success(c, fiber.StatusOK, "Addresses retrieved", fiber.Map{
		"user_id":   userID,
		"addresses": []fiber.Map{},
	})
}

type AddressReq struct {
	RecipientName  string `json:"recipient_name"`
	RecipientPhone string `json:"recipient_phone"`
	AddressLine1   string `json:"address_line1"`
	AddressLine2   string `json:"address_line2"`
	Division       string `json:"division"`
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
	if req.RecipientName == "" || req.RecipientPhone == "" || req.AddressLine1 == "" || req.Division == "" || req.District == "" {
		return response.ValidationError(c, map[string]string{
			"required": "recipient_name, recipient_phone, address_line1, division, district are mandatory",
		})
	}
	return response.Created(c, "Address added successfully", fiber.Map{
		"address": req,
	})
}

func (h *Handler) SetDefaultAddress(c *fiber.Ctx) error {
	addressID := c.Params("id")
	return response.Success(c, fiber.StatusOK, "Default address set", fiber.Map{
		"address_id": addressID,
	})
}

func (h *Handler) DeleteAddress(c *fiber.Ctx) error {
	addressID := c.Params("id")
	return response.Success(c, fiber.StatusOK, "Address deleted successfully", fiber.Map{
		"deleted_id": addressID,
	})
}

type KYCSubmitReq struct {
	DocumentType   string `json:"document_type"` // NID, PASSPORT, DRIVING_LICENSE, TRADE_LICENSE
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
	return response.Success(c, fiber.StatusOK, "KYC documents submitted for verification", fiber.Map{
		"status": "PENDING",
	})
}

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
	return response.Success(c, fiber.StatusOK, "KYC reviewed successfully", fiber.Map{
		"target_user_id": targetUserID,
		"new_status":     req.Status,
	})
}
