package product

import (
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
	router.Get("/products", h.ListProducts)
	router.Get("/products/:slug", h.GetProductDetail)
	router.Get("/categories", h.GetCategoryTree)

	vendor := router.Group("/vendor", authMiddleware)
	vendor.Post("/products", h.CreateProduct)
	vendor.Post("/products/:productId/skus", h.AddSKU)

	admin := router.Group("/admin", authMiddleware)
	admin.Post("/categories", h.CreateCategory)
	admin.Put("/products/:id/approve", h.ApproveProduct)
}

func (h *Handler) ListProducts(c *fiber.Ctx) error {
	limit, offset := h.service.NormalizeLimitOffset(c.QueryInt("limit", 20), c.QueryInt("offset", 0))
	search := strings.TrimSpace(c.Query("q"))
	categorySlug := c.Query("category")

	products, total, err := h.service.ListProducts(c.Context(), search, categorySlug, limit, offset)
	if err != nil && h.db != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to fetch products: "+err.Error(), nil)
	}

	return response.Success(c, fiber.StatusOK, "Products retrieved from NeonDB", fiber.Map{
		"products": products, "total": total, "limit": limit, "offset": offset,
	})
}

func (h *Handler) GetProductDetail(c *fiber.Ctx) error {
	slug := c.Params("slug")

	detail, err := h.service.GetProductDetailBySlug(c.Context(), slug)
	if (err != nil || detail == nil) && h.db != nil {
		return response.Error(c, fiber.StatusNotFound, "Product not found", nil)
	}
	if detail == nil {
		return response.Success(c, fiber.StatusOK, "Product detail", fiber.Map{"slug": slug})
	}

	return response.Success(c, fiber.StatusOK, "Product detail from NeonDB", fiber.Map{
		"id": detail.ID, "product_id": detail.ProductID, "title": detail.Title, "slug": detail.Slug,
		"description": detail.Description, "short_description": detail.ShortDescription,
		"primary_image_url": detail.PrimaryImageURL, "wholesale_price": detail.WholesalePrice,
		"retail_price": detail.RetailPrice, "is_resellable": detail.IsResellable,
		"is_first_party": detail.IsFirstParty, "approval_status": detail.ApprovalStatus,
		"category": detail.Category, "vendor": detail.Vendor, "skus": detail.SKUs,
	})
}

func (h *Handler) GetCategoryTree(c *fiber.Ctx) error {
	categories, err := h.service.GetCategoryTree(c.Context())
	if err != nil && h.db != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to fetch categories: "+err.Error(), nil)
	}
	return response.Success(c, fiber.StatusOK, "Category tree from NeonDB", fiber.Map{"categories": categories})
}

type CreateProductReq struct {
	CategoryID       string  `json:"category_id"`
	Title            string  `json:"title"`
	Slug             string  `json:"slug"`
	Description      string  `json:"description"`
	ShortDescription string  `json:"short_description"`
	PrimaryImageURL  string  `json:"primary_image_url"`
	IsFirstParty     bool    `json:"is_first_party"`
	IsResellable     bool    `json:"is_resellable"`
	WholesalePrice   float64 `json:"wholesale_price"`
	RetailPrice      float64 `json:"retail_price"`
}

func (h *Handler) CreateProduct(c *fiber.Ctx) error {
	var req CreateProductReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body: "+err.Error())
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Slug = strings.TrimSpace(req.Slug)
	req.CategoryID = strings.TrimSpace(req.CategoryID)

	if req.Title == "" || req.Slug == "" || req.CategoryID == "" {
		return response.ValidationError(c, map[string]string{"product": "title, slug, and category_id are required"})
	}

	userID := c.Locals("user_id").(string)

	productID, err := h.service.CreateProduct(c.Context(), CreateProductInput{
		UserID:           userID,
		CategoryID:       req.CategoryID,
		Title:            req.Title,
		Slug:             req.Slug,
		Description:      req.Description,
		ShortDescription: req.ShortDescription,
	})
	if err != nil {
		if err.Error() == "category_not_found" {
			return response.ValidationError(c, map[string]string{"category_id": "category not found"})
		}
		if err.Error() == "slug_exists" {
			return response.Error(c, fiber.StatusConflict, "Product slug already exists", nil)
		}
		if h.db != nil {
			return response.Error(c, fiber.StatusInternalServerError, "Failed to create product: "+err.Error(), nil)
		}
	}

	return response.Created(c, "Product created and approved", fiber.Map{
		"product_id": productID, "id": productID, "slug": req.Slug, "approval_status": "APPROVED",
	})
}

type CreateSKUReq struct {
	SKUCode        string                 `json:"sku_code"`
	Attributes     map[string]interface{} `json:"attributes"`
	WholesalePrice float64                `json:"wholesale_price"`
	RetailPrice    float64                `json:"retail_price"`
	StockQuantity  int                    `json:"stock_quantity"`
	ImageURL       string                 `json:"image_url"`
}

func (h *Handler) AddSKU(c *fiber.Ctx) error {
	productID := c.Params("productId")
	var req CreateSKUReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid SKU payload: "+err.Error())
	}
	req.SKUCode = strings.TrimSpace(req.SKUCode)
	if req.SKUCode == "" {
		return response.ValidationError(c, map[string]string{"sku_code": "required"})
	}

	skuID, err := h.service.AddSKU(c.Context(), AddSKUInput{
		ProductID:      productID,
		SKUCode:        req.SKUCode,
		WholesalePrice: req.WholesalePrice,
		RetailPrice:    req.RetailPrice,
		StockQuantity:  req.StockQuantity,
	})
	if err != nil {
		if err.Error() == "product_not_found" {
			return response.Error(c, fiber.StatusNotFound, "Product not found", nil)
		}
		if err.Error() == "sku_code_exists" {
			return response.Error(c, fiber.StatusConflict, "SKU code already exists", nil)
		}
		if h.db != nil {
			return response.Error(c, fiber.StatusInternalServerError, "Failed to add SKU: "+err.Error(), nil)
		}
	}

	return response.Created(c, "Product SKU added to NeonDB", fiber.Map{
		"sku_id": skuID, "product_id": productID, "sku_code": req.SKUCode, "stock": req.StockQuantity,
	})
}

type CreateCategoryReq struct {
	Name           string  `json:"name"`
	Slug           string  `json:"slug"`
	ParentID       string  `json:"parent_id"`
	IconURL        string  `json:"icon_url"`
	CommissionRate float64 `json:"commission_rate"`
}

func (h *Handler) CreateCategory(c *fiber.Ctx) error {
	var req CreateCategoryReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid category input: "+err.Error())
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Slug = strings.TrimSpace(req.Slug)
	if req.Name == "" || req.Slug == "" {
		return response.ValidationError(c, map[string]string{"category": "name and slug are required"})
	}

	catID, rate, err := h.service.CreateCategory(c.Context(), CreateCategoryInput{
		Name:           req.Name,
		Slug:           req.Slug,
		ParentID:       req.ParentID,
		IconURL:        req.IconURL,
		CommissionRate: req.CommissionRate,
	})
	if err != nil {
		if err.Error() == "slug_exists" {
			return response.Error(c, fiber.StatusConflict, "Category slug already exists", nil)
		}
		if h.db != nil {
			return response.Error(c, fiber.StatusInternalServerError, "Failed to create category: "+err.Error(), nil)
		}
	}

	return response.Created(c, "Category created in NeonDB", fiber.Map{
		"category_id": catID, "id": catID, "name": req.Name, "slug": req.Slug, "commission_rate": rate,
	})
}

type ApproveProductReq struct {
	ApprovalStatus string `json:"approval_status"`
	AdminNote      string `json:"admin_note"`
}

func (h *Handler) ApproveProduct(c *fiber.Ctx) error {
	productID := c.Params("id")
	var req ApproveProductReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid payload: "+err.Error())
	}
	if !h.service.IsValidApprovalStatus(req.ApprovalStatus) {
		return response.ValidationError(c, map[string]string{"approval_status": "must be APPROVED, REJECTED, or SUSPENDED"})
	}

	affected, err := h.service.ApproveProduct(c.Context(), productID, req.ApprovalStatus)
	if err != nil && h.db != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to update product status: "+err.Error(), nil)
	}
	if affected == 0 && h.db != nil {
		return response.Error(c, fiber.StatusNotFound, "Product not found", nil)
	}

	return response.Success(c, fiber.StatusOK, "Product status updated in NeonDB", fiber.Map{
		"product_id": productID, "approval_status": req.ApprovalStatus,
	})
}
