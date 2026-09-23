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
	// Public reseller micro-store view
	router.Get("/reseller/store/:slug", h.GetPublicStore)

	// Reseller portal
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
	return response.Success(c, fiber.StatusOK, "Reseller storefront retrieved", fiber.Map{
		"store_slug": slug,
		"store_name": "Reseller Micro Shop",
		"products":   []fiber.Map{},
	})
}

func (h *Handler) GetMyStore(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	return response.Success(c, fiber.StatusOK, "Reseller store details", fiber.Map{
		"user_id":              userID,
		"total_profit_earned":  4580.00,
		"wallet_balance":       1200.00,
	})
}

type CreateStoreReq struct {
	StoreName    string `json:"store_name"`
	StoreSlug    string `json:"store_slug"`
	CustomDomain string `json:"custom_domain"`
}

func (h *Handler) CreateStore(c *fiber.Ctx) error {
	var req CreateStoreReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}
	if req.StoreName == "" || req.StoreSlug == "" {
		return response.ValidationError(c, map[string]string{
			"store": "store_name and store_slug are required",
		})
	}
	return response.Created(c, "Reseller zero-inventory store created successfully!", fiber.Map{
		"store_slug": req.StoreSlug,
		"share_url":  fmt.Sprintf("https://reseller.platform.com/s/%s", req.StoreSlug),
	})
}

func (h *Handler) GetCatalog(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Reseller catalog list", fiber.Map{
		"catalog": []fiber.Map{},
	})
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
	if req.ProductID == "" || req.ResellerMargin < 0 {
		return response.BadRequest(c, "Valid product_id and non-negative margin required")
	}

	finalSellingPrice := req.WholesalePrice + req.ResellerMargin

	return response.Success(c, fiber.StatusOK, "Product added to reseller catalog with custom profit margin", fiber.Map{
		"product_id":          req.ProductID,
		"wholesale_price":     req.WholesalePrice,
		"reseller_margin":     req.ResellerMargin,
		"final_selling_price": finalSellingPrice,
	})
}

func (h *Handler) RemoveFromCatalog(c *fiber.Ctx) error {
	productID := c.Params("productId")
	return response.Success(c, fiber.StatusOK, "Product removed from reseller catalog", fiber.Map{
		"removed_product_id": productID,
	})
}

type CalculateMarginReq struct {
	WholesalePrice float64 `json:"wholesale_price"`
	TargetPrice    float64 `json:"target_price"`
}

func (h *Handler) CalculateMargin(c *fiber.Ctx) error {
	var req CalculateMarginReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request: "+err.Error())
	}
	margin := req.TargetPrice - req.WholesalePrice
	percentage := 0.0
	if req.WholesalePrice > 0 {
		percentage = (margin / req.WholesalePrice) * 100
	}
	return response.Success(c, fiber.StatusOK, "Margin calculation result", fiber.Map{
		"wholesale_price":  req.WholesalePrice,
		"target_price":     req.TargetPrice,
		"expected_profit":  margin,
		"profit_percentage": fmt.Sprintf("%.2f%%", percentage),
	})
}

type ShareLinkReq struct {
	ProductID string `json:"product_id"`
	Channel   string `json:"channel"` // WhatsApp, Facebook, IMO
}

func (h *Handler) GenerateShareableLink(c *fiber.Ctx) error {
	var req ShareLinkReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request: "+err.Error())
	}
	userID := c.Locals("user_id").(string)
	uLen := len(userID)
	if uLen > 8 {
		uLen = 8
	}
	pLen := len(req.ProductID)
	if pLen > 8 {
		pLen = 8
	}
	refCode := fmt.Sprintf("RES-%s-%s", userID[:uLen], req.ProductID[:pLen])

	return response.Success(c, fiber.StatusOK, "White-labeled product share link generated", fiber.Map{
		"share_link": fmt.Sprintf("https://buy.platform.com/p/%s?ref=%s", req.ProductID, refCode),
		"ref_code":   refCode,
		"channel":    req.Channel,
	})
}
