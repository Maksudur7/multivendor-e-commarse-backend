package product

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
	// Public catalog APIs
	router.Get("/products", h.ListProducts)
	router.Get("/products/:slug", h.GetProductDetail)
	router.Get("/categories", h.GetCategoryTree)

	// Vendor/First-party product creation
	v := router.Group("/vendor/products", authMiddleware)
	v.Post("/", h.CreateProduct)
	v.Post("/:productId/skus", h.AddSKU)

	// Admin product approvals & categories
	admin := router.Group("/admin", authMiddleware)
	admin.Post("/categories", h.CreateCategory)
	admin.Put("/products/:id/approve", h.ApproveProduct)
}

func (h *Handler) ListProducts(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 20)
	offset := c.QueryInt("offset", 0)

	return response.Success(c, fiber.StatusOK, "Products list retrieved", fiber.Map{
		"limit":    limit,
		"offset":   offset,
		"products": []fiber.Map{},
	})
}

func (h *Handler) GetProductDetail(c *fiber.Ctx) error {
	slug := c.Params("slug")
	return response.Success(c, fiber.StatusOK, "Product details retrieved", fiber.Map{
		"slug":             slug,
		"title":            "Sample Enterprise Product",
		"wholesale_price":  500.00,
		"retail_price":     750.00,
		"is_resellable":    true,
		"skus":             []fiber.Map{},
	})
}

func (h *Handler) GetCategoryTree(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Category hierarchy tree", fiber.Map{
		"categories": []fiber.Map{},
	})
}

type CreateProductReq struct {
	CategoryID       string  `json:"category_id"`
	BrandID          string  `json:"brand_id"`
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
	if req.Title == "" || req.Slug == "" || req.CategoryID == "" {
		return response.ValidationError(c, map[string]string{
			"product": "title, slug, and category_id are mandatory fields",
		})
	}
	return response.Created(c, "Product created and submitted for admin review", fiber.Map{
		"slug":            req.Slug,
		"approval_status": "PENDING",
	})
}

type CreateSKUReq struct {
	SKUCode        string                 `json:"sku_code"`
	Attributes     map[string]interface{} `json:"attributes"` // e.g. {"color": "Red", "size": "XL"}
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
	return response.Created(c, "Product SKU variant added", fiber.Map{
		"product_id": productID,
		"sku_code":   req.SKUCode,
		"stock":      req.StockQuantity,
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
	return response.Created(c, "Category created", fiber.Map{
		"name": req.Name,
		"slug": req.Slug,
	})
}

type ApproveProductReq struct {
	ApprovalStatus string `json:"approval_status"` // APPROVED, REJECTED
}

func (h *Handler) ApproveProduct(c *fiber.Ctx) error {
	productID := c.Params("id")
	var req ApproveProductReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid payload: "+err.Error())
	}
	return response.Success(c, fiber.StatusOK, "Product status updated", fiber.Map{
		"product_id":      productID,
		"approval_status": req.ApprovalStatus,
	})
}
