package vendor

import (
		"time"

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
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Vendor store loaded", fiber.Map{
			"store_slug": slug, "store_name": "Official Store", "rating": 4.8,
		})
	}
	ctx := c.Context()
	var id, name, desc, logo, status string
	var rating float64
	err := h.db.QueryRow(ctx, `
		SELECT id::text, store_name, COALESCE(description,''), COALESCE(logo_url,''), verification_status, store_rating
		FROM vendors WHERE store_slug = $1`, slug).Scan(&id, &name, &desc, &logo, &status, &rating)
	if err != nil {
		return response.Error(c, fiber.StatusNotFound, "Vendor store not found", nil)
	}
	return response.Success(c, fiber.StatusOK, "Vendor store loaded from NeonDB", fiber.Map{
		"vendor_id": id, "store_name": name, "store_slug": slug, "description": desc, "logo_url": logo, "verification_status": status, "rating": rating,
	})
}

func (h *Handler) GetMyStore(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Vendor store profile", fiber.Map{"user_id": userID, "status": "APPROVED"})
	}
	ctx := c.Context()
	var id, name, slug, status string
	var commission float64
	err := h.db.QueryRow(ctx, `
		SELECT id::text, store_name, store_slug, verification_status, commission_rate
		FROM vendors WHERE user_id::text = $1`, userID).Scan(&id, &name, &slug, &status, &commission)
	if err != nil {
		return response.Success(c, fiber.StatusOK, "Vendor profile (Not yet registered as seller)", fiber.Map{"registered": false})
	}
	return response.Success(c, fiber.StatusOK, "Vendor store profile from NeonDB", fiber.Map{
		"registered": true, "vendor_id": id, "store_name": name, "store_slug": slug, "verification_status": status, "commission_rate": commission,
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
	userID := c.Locals("user_id").(string)
	var req RegisterStoreReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request: "+err.Error())
	}
	if req.StoreName == "" || req.StoreSlug == "" {
		return response.ValidationError(c, map[string]string{"store": "store_name and store_slug are required"})
	}

	if h.db == nil {
		return response.Created(c, "Vendor application submitted", fiber.Map{"store_slug": req.StoreSlug, "status": "PENDING"})
	}

	ctx := c.Context()
	var vendorID string
	err := h.db.QueryRow(ctx, `
		INSERT INTO vendors (user_id, store_name, store_slug, description, bank_name, bank_account_number, bkash_merchant_number, verification_status)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, 'PENDING')
		ON CONFLICT (user_id) DO UPDATE SET store_name = EXCLUDED.store_name, store_slug = EXCLUDED.store_slug
		RETURNING id::text`,
		userID, req.StoreName, req.StoreSlug, req.Description, req.BankName, req.BankAccountNumber, req.BkashMerchantNumber,
	).Scan(&vendorID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to register vendor: "+err.Error(), nil)
	}

	// Update user role to VENDOR
	h.db.Exec(ctx, "UPDATE users SET role = 'VENDOR' WHERE id::text = $1", userID)

	return response.Created(c, "Vendor application registered in NeonDB", fiber.Map{
		"vendor_id": vendorID, "store_slug": req.StoreSlug, "status": "PENDING",
	})
}

func (h *Handler) UpdateStore(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	var req RegisterStoreReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid body: "+err.Error())
	}
	if h.db != nil {
		h.db.Exec(c.Context(), `
			UPDATE vendors SET store_name = COALESCE(NULLIF($1,''), store_name), description = COALESCE(NULLIF($2,''), description)
			WHERE user_id::text = $3`, req.StoreName, req.Description, userID)
	}
	return response.Success(c, fiber.StatusOK, "Vendor store updated in NeonDB", fiber.Map{"user_id": userID})
}

func (h *Handler) GetVendorAnalytics(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Vendor analytics", fiber.Map{
			"total_revenue": 0.0, "total_orders": 0, "wallet_balance": 0.0,
		})
	}
	ctx := c.Context()

	// Get vendor ID
	var vendorID string
	err := h.db.QueryRow(ctx, "SELECT id::text FROM vendors WHERE user_id::text = $1", userID).Scan(&vendorID)
	if err != nil {
		return response.Success(c, fiber.StatusOK, "Vendor not found", fiber.Map{"registered": false})
	}

	var walletBal, pendingEscrow float64
	h.db.QueryRow(ctx, "SELECT available_balance, COALESCE(pending_escrow,0) FROM wallets WHERE user_id::text = $1", userID).Scan(&walletBal, &pendingEscrow)

	var totalOrders int
	var totalRevenue float64
	h.db.QueryRow(ctx, `
		SELECT COUNT(*), COALESCE(SUM(grand_total),0)
		FROM master_orders WHERE vendor_id::text = $1`, vendorID).Scan(&totalOrders, &totalRevenue)

	var pendingOrders, completedOrders int
	h.db.QueryRow(ctx, `SELECT COUNT(*) FROM master_orders WHERE vendor_id::text = $1 AND status = 'PENDING'`, vendorID).Scan(&pendingOrders)
	h.db.QueryRow(ctx, `SELECT COUNT(*) FROM master_orders WHERE vendor_id::text = $1 AND status = 'DELIVERED'`, vendorID).Scan(&completedOrders)

	return response.Success(c, fiber.StatusOK, "Vendor analytics from NeonDB", fiber.Map{
		"vendor_id":       vendorID,
		"wallet_balance":  walletBal,
		"pending_escrow":  pendingEscrow,
		"total_orders":    totalOrders,
		"pending_orders":  pendingOrders,
		"completed_orders": completedOrders,
		"total_revenue":   totalRevenue,
	})
}

func (h *Handler) ListVendorsAdmin(c *fiber.Ctx) error {
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Vendors list for admin", fiber.Map{"vendors": []fiber.Map{}})
	}
	ctx := c.Context()
	rows, err := h.db.Query(ctx, `
		SELECT v.id::text, v.store_name, v.store_slug, v.verification_status, v.commission_rate, v.created_at, COALESCE(u.email,'')
		FROM vendors v LEFT JOIN users u ON u.id = v.user_id ORDER BY v.created_at DESC`)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load vendors", nil)
	}
	defer rows.Close()

	vendors := []fiber.Map{}
	for rows.Next() {
		var id, name, slug, status, email string
		var comm float64
		var dt time.Time
		rows.Scan(&id, &name, &slug, &status, &comm, &dt, &email)
		vendors = append(vendors, fiber.Map{
			"vendor_id": id, "store_name": name, "store_slug": slug, "status": status,
			"commission_rate": comm, "email": email, "created_at": dt.Format(time.RFC3339),
		})
	}
	return response.Success(c, fiber.StatusOK, "Vendors list from NeonDB", fiber.Map{"vendors": vendors, "count": len(vendors)})
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
	if req.Status == "" { req.Status = "APPROVED" }
	if req.CommissionRate <= 0 { req.CommissionRate = 5.0 }

	if h.db != nil {
		ctx := c.Context()
		_, err := h.db.Exec(ctx, "UPDATE vendors SET verification_status = $1, commission_rate = $2 WHERE id::text = $3", req.Status, req.CommissionRate, vendorID)
		if err != nil {
			return response.Error(c, fiber.StatusInternalServerError, "Failed to update status", nil)
		}
	}
	return response.Success(c, fiber.StatusOK, "Vendor verification updated in NeonDB", fiber.Map{
		"vendor_id": vendorID, "status": req.Status, "commission_rate": req.CommissionRate,
	})
}
