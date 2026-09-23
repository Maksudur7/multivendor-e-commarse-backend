package vendor

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yourusername/ecom-backend/pkg/response"
)

type Handler struct {
	db *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{db: db}
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
	return response.Success(c, fiber.StatusOK, "Vendor store loaded", fiber.Map{
		"store_slug": slug,
		"store_name": "Sample Official Store",
		"rating":     4.8,
		"products":   []string{},
	})
}

func (h *Handler) GetMyStore(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	return response.Success(c, fiber.StatusOK, "Vendor store profile", fiber.Map{
		"user_id": userID,
		"status":  "APPROVED",
	})
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
	var req RegisterStoreReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request: "+err.Error())
	}
	if req.StoreName == "" || req.StoreSlug == "" {
		return response.ValidationError(c, map[string]string{
			"store": "store_name and store_slug are required",
		})
	}
	return response.Created(c, "Vendor application submitted for admin review", fiber.Map{
		"store_slug": req.StoreSlug,
		"status":     "PENDING",
	})
}

func (h *Handler) UpdateStore(c *fiber.Ctx) error {
	var req RegisterStoreReq
	_ = c.BodyParser(&req)
	return response.Success(c, fiber.StatusOK, "Store updated", fiber.Map{
		"store": req,
	})
}

func (h *Handler) GetVendorAnalytics(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Vendor analytics", fiber.Map{
		"total_revenue":      154200.00,
		"total_orders":       312,
		"pending_payout":     12400.00,
		"escrow_locked":      18000.00,
		"commission_paid":    7710.00,
	})
}

func (h *Handler) ListVendorsAdmin(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Vendors list for admin", fiber.Map{
		"vendors": []fiber.Map{},
	})
}

type VerifyVendorReq struct {
	Status         string  `json:"status"` // APPROVED, REJECTED, SUSPENDED
	CommissionRate float64 `json:"commission_rate"`
}

func (h *Handler) VerifyVendor(c *fiber.Ctx) error {
	vendorID := c.Params("id")
	var req VerifyVendorReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}
	return response.Success(c, fiber.StatusOK, "Vendor verification status updated", fiber.Map{
		"vendor_id":       vendorID,
		"status":          req.Status,
		"commission_rate": req.CommissionRate,
	})
}
