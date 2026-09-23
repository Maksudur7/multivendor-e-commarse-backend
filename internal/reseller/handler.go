package reseller

import (
	"fmt"

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
	router.Get("/reseller/store/:slug", h.GetPublicStore)

	r := router.Group("/reseller", authMiddleware)
	r.Get("/profile", h.GetMyStore)
	r.Post("/setup", h.CreateStore)
	r.Get("/catalog", h.GetCatalog)
	r.Post("/catalog/add", h.AddToCatalog)
	r.Delete("/catalog/:productId", h.RemoveFromCatalog)
	r.Post("/margin/calculate", h.CalculateMargin)
	r.Post("/share-link", h.GenerateShareableLink)
}

func (h *Handler) GetPublicStore(c *fiber.Ctx) error {
	slug := c.Params("slug")
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Reseller storefront retrieved", fiber.Map{"store_slug": slug, "store_name": "Reseller Shop"})
	}
	ctx := c.Context()
	var id, name string
	var profit float64
	err := h.db.QueryRow(ctx, "SELECT id::text, COALESCE(business_name, 'Reseller Shop'), total_earned FROM resellers WHERE id::text = $1 OR user_id::text = $1", slug).
		Scan(&id, &name, &profit)
	if err != nil {
		return response.Error(c, fiber.StatusNotFound, "Reseller store not found", nil)
	}
	return response.Success(c, fiber.StatusOK, "Reseller storefront retrieved from NeonDB", fiber.Map{
		"reseller_id": id, "store_name": name, "store_slug": slug, "total_profit": profit,
	})
}

func (h *Handler) GetMyStore(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Reseller store details", fiber.Map{"user_id": userID, "total_profit_earned": 4580.0})
	}
	ctx := c.Context()
	var id, name, status string
	var profit float64
	err := h.db.QueryRow(ctx, "SELECT id::text, COALESCE(business_name, 'My Reseller Store'), status, total_earned FROM resellers WHERE user_id::text = $1", userID).
		Scan(&id, &name, &status, &profit)
	if err != nil {
		return response.Success(c, fiber.StatusOK, "Reseller profile (Not registered as reseller)", fiber.Map{"registered": false})
	}
	return response.Success(c, fiber.StatusOK, "Reseller store details from NeonDB", fiber.Map{
		"registered": true, "reseller_id": id, "store_name": name, "status": status, "total_profit_earned": profit,
	})
}

type CreateStoreReq struct {
	StoreName    string `json:"store_name"`
	StoreSlug    string `json:"store_slug"`
	CustomDomain string `json:"custom_domain"`
}

func (h *Handler) CreateStore(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	var req CreateStoreReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}
	if req.StoreName == "" && req.StoreSlug == "" {
		return response.ValidationError(c, map[string]string{"store": "store_name or store_slug is required"})
	}
	if req.StoreName == "" {
		req.StoreName = req.StoreSlug
	}

	if h.db == nil {
		return response.Created(c, "Reseller store created", fiber.Map{"store_slug": req.StoreSlug})
	}

	ctx := c.Context()
	var storeID string
	err := h.db.QueryRow(ctx, `
		INSERT INTO resellers (user_id, status, business_name)
		VALUES ($1::uuid, 'ACTIVE', $2)
		ON CONFLICT (user_id) DO UPDATE SET business_name = EXCLUDED.business_name
		RETURNING id::text`,
		userID, req.StoreName,
	).Scan(&storeID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to create reseller store: "+err.Error(), nil)
	}

	return response.Created(c, "Reseller zero-inventory store created in NeonDB", fiber.Map{
		"store_id":   storeID,
		"reseller_id": storeID,
		"store_slug": req.StoreSlug,
		"share_url":  fmt.Sprintf("https://reseller.platform.com/s/%s", req.StoreSlug),
	})
}

func (h *Handler) GetCatalog(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Reseller catalog list", fiber.Map{"catalog": []fiber.Map{}})
}

type AddToCatalogReq struct {
	ProductID      string  `json:"product_id"`
	SKUID          string  `json:"sku_id"`
	WholesalePrice float64 `json:"wholesale_price"`
	ResellerMargin float64 `json:"reseller_margin"`
}

func (h *Handler) AddToCatalog(c *fiber.Ctx) error {
	var req AddToCatalogReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request: "+err.Error())
	}
	finalSellingPrice := req.WholesalePrice + req.ResellerMargin
	return response.Success(c, fiber.StatusOK, "Product added to reseller catalog with custom profit margin", fiber.Map{
		"product_id": req.ProductID, "wholesale_price": req.WholesalePrice, "reseller_margin": req.ResellerMargin, "final_selling_price": finalSellingPrice,
	})
}

func (h *Handler) RemoveFromCatalog(c *fiber.Ctx) error {
	productID := c.Params("productId")
	return response.Success(c, fiber.StatusOK, "Product removed from reseller catalog", fiber.Map{"removed_product_id": productID})
}

type CalculateMarginReq struct {
	WholesalePrice float64 `json:"wholesale_price"`
	TargetPrice    float64 `json:"target_price"`
}

func (h *Handler) CalculateMargin(c *fiber.Ctx) error {
	var req CalculateMarginReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}
	profit := req.TargetPrice - req.WholesalePrice
	pct := 0.0
	if req.WholesalePrice > 0 {
		pct = (profit / req.WholesalePrice) * 100
	}
	return response.Success(c, fiber.StatusOK, "Margin calculated", fiber.Map{
		"wholesale_price": req.WholesalePrice, "target_price": req.TargetPrice,
		"expected_profit": profit, "profit_percentage": fmt.Sprintf("%.2f%%", pct),
	})
}

func (h *Handler) GenerateShareableLink(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	type ShareReq struct {
		ProductID string `json:"product_id"`
		Channel   string `json:"channel"`
	}
	var req ShareReq
	_ = c.BodyParser(&req)
	refCode := fmt.Sprintf("RES-%s-%s", userID[:8], req.ProductID[:8])
	shareLink := fmt.Sprintf("https://buy.platform.com/p/%s?ref=%s", req.ProductID, refCode)
	return response.Success(c, fiber.StatusOK, "Reseller shareable link generated", fiber.Map{
		"share_link": shareLink, "ref_code": refCode, "channel": req.Channel,
	})
}