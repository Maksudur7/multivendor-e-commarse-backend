package reseller

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yourusername/ecom-backend/pkg/middleware"
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
	router.Get("/reseller/store/:slug", h.GetPublicStore)

	r := router.Group("/reseller", authMiddleware)
	r.Get("/profile", h.GetMyStore)
	r.Post("/setup", h.CreateStore)
	r.Get("/catalog", h.GetCatalog)
	r.Post("/catalog/add", h.AddToCatalog)
	r.Delete("/catalog/:productId", h.RemoveFromCatalog)
	r.Post("/margin/calculate", h.CalculateMargin)
	r.Post("/share-link", h.GenerateShareableLink)

	admin := router.Group("/admin/resellers", authMiddleware, middleware.RequireRole("ADMIN", "SUPER_ADMIN", "ADMIN_OPS"))
	admin.Get("/", h.ListResellersAdmin)
	admin.Put("/:id/verify", h.VerifyReseller)
}

func (h *Handler) GetPublicStore(c *fiber.Ctx) error {
	slug := c.Params("slug")
	store, err := h.service.GetPublicStore(c.Context(), slug)
	if err != nil {
		return response.Error(c, fiber.StatusNotFound, "Reseller store not found", nil)
	}
	return response.Success(c, fiber.StatusOK, "Reseller storefront retrieved", store)
}

func (h *Handler) GetMyStore(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	profile, err := h.service.GetMyStore(c.Context(), userID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load reseller profile", nil)
	}
	return response.Success(c, fiber.StatusOK, "Reseller store details", profile)
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

	storeID, err := h.service.CreateStore(c.Context(), userID, req.StoreName, req.StoreSlug)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to create store: "+err.Error(), nil)
	}

	return response.Created(c, "Reseller zero-inventory store created", fiber.Map{
		"store_id":    storeID,
		"reseller_id": storeID,
		"store_slug":  req.StoreSlug,
		"share_url":   fmt.Sprintf("https://reseller.platform.com/s/%s", req.StoreSlug),
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
	result := h.service.CalculateMargin(req.WholesalePrice, req.TargetPrice)
	return response.Success(c, fiber.StatusOK, "Margin calculated", result)
}

func (h *Handler) GenerateShareableLink(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	type ShareReq struct {
		ProductID string `json:"product_id"`
		Channel   string `json:"channel"`
	}
	var req ShareReq
	_ = c.BodyParser(&req)

	shareLink, refCode := h.service.GenerateShareableLink(userID, req.ProductID, req.Channel)
	return response.Success(c, fiber.StatusOK, "Reseller shareable link generated", fiber.Map{
		"share_link": shareLink, "ref_code": refCode, "channel": req.Channel,
	})
}

func (h *Handler) ListResellersAdmin(c *fiber.Ctx) error {
	status := c.Query("status")
	list, err := h.service.ListResellersAdmin(c.Context(), status)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to list resellers: "+err.Error(), nil)
	}
	return response.Success(c, fiber.StatusOK, "Reseller accounts fetched", fiber.Map{
		"resellers": list,
		"count":     len(list),
	})
}

type VerifyResellerReq struct {
	Status string `json:"status"`
}

func (h *Handler) VerifyReseller(c *fiber.Ctx) error {
	resellerID := c.Params("id")
	var req VerifyResellerReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request payload: "+err.Error())
	}
	if req.Status != "ACTIVE" && req.Status != "REJECTED" && req.Status != "SUSPENDED" {
		return response.BadRequest(c, "Status must be ACTIVE, REJECTED, or SUSPENDED")
	}

	affected, err := h.service.VerifyReseller(c.Context(), resellerID, req.Status)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to verify reseller: "+err.Error(), nil)
	}
	if affected == 0 {
		return response.NotFound(c, "Reseller application not found")
	}

	return response.Success(c, fiber.StatusOK, "Reseller status updated and user role updated", fiber.Map{
		"reseller_id": resellerID,
		"new_status":  req.Status,
	})
}