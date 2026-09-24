package vendor

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yourusername/ecom-backend/pkg/response"
)

type Handler struct {
	repo    *Repository
	service *Service
}

func NewHandler(db *pgxpool.Pool) *Handler {
	repo := NewRepository(db)
	service := NewService(repo)
	return &Handler{repo: repo, service: service}
}

func (h *Handler) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	// Public store front view
	router.Get("/vendors/store/:slug", h.GetStorePublic)

	// Vendor user portal
	v := router.Group("/vendor", authMiddleware)
	v.Get("/profile", h.GetMyStore)
	v.Post("/register", h.RegisterStore)
	v.Put("/profile", h.UpdateStore)
	v.Get("/analytics", h.GetVendorAnalytics)

	// Admin control
	admin := router.Group("/admin/vendors", authMiddleware)
	admin.Get("/", h.ListVendorsAdmin)
	admin.Put("/:id/verify", h.VerifyVendor)
}

func (h *Handler) GetStorePublic(c *fiber.Ctx) error {
	slug := c.Params("slug")
	store, err := h.service.GetStorePublic(c.Context(), slug)
	if err != nil {
		return response.Error(c, fiber.StatusNotFound, "Vendor store not found", nil)
	}
	return response.Success(c, fiber.StatusOK, "Vendor store loaded", store)
}

func (h *Handler) GetMyStore(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	vp, err := h.service.GetMyStore(c.Context(), userID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load profile", nil)
	}
	return response.Success(c, fiber.StatusOK, "Vendor store profile", vp)
}

type RegisterStoreReq struct {
	StoreName           string `json:"store_name"`
	StoreSlug           string `json:"store_slug"`
	Description         string `json:"description"`
	BankName            string `json:"bank_name"`
	BankAccountNumber   string `json:"bank_account_number"`
	BankRoutingNumber   string `json:"bank_routing_number"`
	BkashMerchantNumber string `json:"bkash_merchant_number"`
}

func (h *Handler) RegisterStore(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	var req RegisterStoreReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request: "+err.Error())
	}
	if req.StoreName == "" || req.StoreSlug == "" {
		return response.ValidationError(c, map[string]string{"store": "store_name and store_slug are required"})
	}

	params := RegisterStoreParams{
		UserID:              userID,
		StoreName:           req.StoreName,
		StoreSlug:           req.StoreSlug,
		Description:         req.Description,
		BankName:            req.BankName,
		BankAccountNumber:   req.BankAccountNumber,
		BkashMerchantNumber: req.BkashMerchantNumber,
	}

	vendorID, err := h.service.RegisterStore(c.Context(), params)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to register vendor: "+err.Error(), nil)
	}

	return response.Created(c, "Vendor application registered", fiber.Map{
		"vendor_id": vendorID, "store_slug": req.StoreSlug, "status": "PENDING",
	})
}

func (h *Handler) UpdateStore(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	var req RegisterStoreReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid body: "+err.Error())
	}
	if err := h.service.UpdateStore(c.Context(), userID, req.StoreName, req.Description); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to update vendor", nil)
	}
	return response.Success(c, fiber.StatusOK, "Vendor store updated", fiber.Map{"user_id": userID})
}

func (h *Handler) GetVendorAnalytics(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	analytics, err := h.service.GetVendorAnalytics(c.Context(), userID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load analytics", nil)
	}
	return response.Success(c, fiber.StatusOK, "Vendor analytics loaded", analytics)
}

func (h *Handler) ListVendorsAdmin(c *fiber.Ctx) error {
	vendors, err := h.service.ListVendorsAdmin(c.Context())
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load vendors", nil)
	}
	return response.Success(c, fiber.StatusOK, "Vendors list for admin", fiber.Map{"vendors": vendors, "count": len(vendors)})
}

type VerifyVendorReq struct {
	Status         string  `json:"status"`
	CommissionRate float64 `json:"commission_rate"`
}

func (h *Handler) VerifyVendor(c *fiber.Ctx) error {
	vendorID := c.Params("id")
	var req VerifyVendorReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}

	if err := h.service.VerifyVendor(c.Context(), vendorID, req.Status, req.CommissionRate); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to update verification", nil)
	}

	return response.Success(c, fiber.StatusOK, "Vendor verification updated", fiber.Map{
		"vendor_id": vendorID, "status": req.Status, "commission_rate": req.CommissionRate,
	})
}
