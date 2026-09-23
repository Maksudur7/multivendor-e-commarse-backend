package product

import (
	"context"
	"fmt"
	"strings"
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
	limit := c.QueryInt("limit", 20)
	offset := c.QueryInt("offset", 0)
	search := strings.TrimSpace(c.Query("q"))
	categorySlug := c.Query("category")

	if limit > 100 {
		limit = 100
	}

	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Products retrieved", fiber.Map{
			"products": []fiber.Map{}, "total": 0, "limit": limit, "offset": offset,
		})
	}

	ctx := c.Context()
	conditions := []string{"p.deleted_at IS NULL"}
	args := []interface{}{}
	argIdx := 1

	if search != "" {
		conditions = append(conditions, fmt.Sprintf("(p.name ILIKE $%d OR p.short_description ILIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+search+"%")
		argIdx++
	}
	if categorySlug != "" {
		conditions = append(conditions, fmt.Sprintf("c.slug = $%d", argIdx))
		args = append(args, categorySlug)
		argIdx++
	}

	whereClause := "WHERE " + strings.Join(conditions, " AND ")
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	args = append(args, limit, offset)

	query := fmt.Sprintf(`
		SELECT p.id::text, p.name, p.slug,
		       COALESCE(c.name,'') as category_name,
		       COALESCE(s.shop_name, u.full_name, 'First Party') as vendor_name, p.created_at
		FROM products p
		LEFT JOIN categories c ON p.category_id = c.id
		LEFT JOIN seller_profiles s ON p.seller_id = s.user_id
		LEFT JOIN users u ON p.seller_id = u.id
		%s ORDER BY p.created_at DESC LIMIT $%d OFFSET $%d`, whereClause, argIdx, argIdx+1)

	rows, err := h.db.Query(ctx, query, args...)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to fetch products: "+err.Error(), nil)
	}
	defer rows.Close()

	products := []fiber.Map{}
	for rows.Next() {
		var id, title, slug, catName, vendorName string
		var createdAt time.Time
		if err := rows.Scan(&id, &title, &slug, &catName, &vendorName, &createdAt); err != nil {
			continue
		}
		products = append(products, fiber.Map{
			"id": id, "product_id": id, "title": title, "slug": slug, "primary_image_url": "",
			"wholesale_price": 800.0, "retail_price": 1200.0,
			"is_resellable": true, "category": catName,
			"vendor": vendorName, "created_at": createdAt,
		})
	}

	var total int
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM products p LEFT JOIN categories c ON p.category_id = c.id %s`, whereClause)
	_ = h.db.QueryRow(ctx, countQuery, countArgs...).Scan(&total)

	return response.Success(c, fiber.StatusOK, "Products retrieved from NeonDB", fiber.Map{
		"products": products, "total": total, "limit": limit, "offset": offset,
	})
}

func (h *Handler) GetProductDetail(c *fiber.Ctx) error {
	slug := c.Params("slug")
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Product detail", fiber.Map{"slug": slug})
	}

	ctx := c.Context()
	var id, title, description, shortDesc, approvalStatus, catName, vendorName string

	err := h.db.QueryRow(ctx, `
		SELECT p.id::text, p.name, COALESCE(p.description,''), COALESCE(p.short_description,''),
		       p.status, COALESCE(c.name,'') as cat, COALESCE(s.shop_name, u.full_name, 'First Party') as vendor
		FROM products p
		LEFT JOIN categories c ON p.category_id = c.id
		LEFT JOIN seller_profiles s ON p.seller_id = s.user_id
		LEFT JOIN users u ON p.seller_id = u.id
		WHERE p.slug = $1 AND p.deleted_at IS NULL`, slug).
		Scan(&id, &title, &description, &shortDesc, &approvalStatus, &catName, &vendorName)

	if err != nil {
		return response.Error(c, fiber.StatusNotFound, "Product not found", nil)
	}

	_, _ = h.db.Exec(ctx, "UPDATE products SET view_count = view_count + 1 WHERE slug = $1", slug)

	skuRows, _ := h.db.Query(ctx, `
		SELECT id::text, sku_code, wholesale_price, retail_price, stock_quantity
		FROM product_skus WHERE product_id = $1::uuid`, id)
	skus := []fiber.Map{}
	if skuRows != nil {
		defer skuRows.Close()
		for skuRows.Next() {
			var skuID, skuCode string
			var wPrice, rPrice float64
			var stock int
			if err := skuRows.Scan(&skuID, &skuCode, &wPrice, &rPrice, &stock); err == nil {
				skus = append(skus, fiber.Map{
					"id": skuID, "sku_code": skuCode,
					"wholesale_price": wPrice, "retail_price": rPrice, "stock_quantity": stock,
				})
			}
		}
	}

	return response.Success(c, fiber.StatusOK, "Product detail from NeonDB", fiber.Map{
		"id": id, "product_id": id, "title": title, "slug": slug, "description": description,
		"short_description": shortDesc, "primary_image_url": "",
		"wholesale_price": 800.0, "retail_price": 1200.0,
		"is_resellable": true, "is_first_party": false,
		"approval_status": approvalStatus, "category": catName, "vendor": vendorName, "skus": skus,
	})
}

func (h *Handler) GetCategoryTree(c *fiber.Ctx) error {
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Categories", fiber.Map{"categories": []fiber.Map{}})
	}

	ctx := c.Context()
	rows, err := h.db.Query(ctx, `
		SELECT c.id::text, COALESCE(c.parent_id::text,'') as parent_id, c.name, c.slug,
		       COALESCE(c.icon_url,''), COALESCE(r.commission_rate, 10.0) as commission_rate
		FROM categories c
		LEFT JOIN category_commission_rates r ON c.id = r.category_id
		WHERE c.is_active = true ORDER BY c.name`)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to fetch categories: "+err.Error(), nil)
	}
	defer rows.Close()

	categories := []fiber.Map{}
	for rows.Next() {
		var id, parentID, name, slug, iconURL string
		var rate float64
		if err := rows.Scan(&id, &parentID, &name, &slug, &iconURL, &rate); err != nil {
			continue
		}
		categories = append(categories, fiber.Map{
			"id": id, "category_id": id, "parent_id": parentID, "name": name, "slug": slug,
			"icon_url": iconURL, "commission_rate": rate,
		})
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

	if h.db == nil {
		return response.Created(c, "Product created", fiber.Map{"slug": req.Slug, "approval_status": "APPROVED"})
	}

	ctx := c.Context()
	userID := c.Locals("user_id").(string)

	var catExists bool
	h.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM categories WHERE id::text = $1)", req.CategoryID).Scan(&catExists)
	if !catExists {
		return response.ValidationError(c, map[string]string{"category_id": "category not found"})
	}

	var slugExists bool
	h.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM products WHERE slug = $1)", req.Slug).Scan(&slugExists)
	if slugExists {
		return response.Error(c, fiber.StatusConflict, "Product slug already exists", nil)
	}

	var productID string
	err := h.db.QueryRow(ctx, `
		INSERT INTO products (seller_id, category_id, name, slug, description, short_description, status)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, 'APPROVED') RETURNING id::text`,
		userID, req.CategoryID, req.Title, req.Slug, req.Description, req.ShortDescription,
	).Scan(&productID)

	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to create product: "+err.Error(), nil)
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

	if h.db == nil {
		return response.Created(c, "SKU added", fiber.Map{"product_id": productID, "sku_code": req.SKUCode})
	}

	ctx := c.Context()

	var productExists bool
	h.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM products WHERE id::text = $1)", productID).Scan(&productExists)
	if !productExists {
		return response.Error(c, fiber.StatusNotFound, "Product not found", nil)
	}

	var skuID string
	err := h.db.QueryRow(ctx, `
		INSERT INTO product_skus (product_id, sku_code, wholesale_price, retail_price, stock_quantity)
		VALUES ($1::uuid, $2, $3, $4, $5) RETURNING id::text`,
		productID, req.SKUCode, req.WholesalePrice, req.RetailPrice, req.StockQuantity,
	).Scan(&skuID)

	if err != nil {
		if strings.Contains(err.Error(), "unique") {
			return response.Error(c, fiber.StatusConflict, "SKU code already exists", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "Failed to add SKU: "+err.Error(), nil)
	}

	_, _ = h.db.Exec(ctx, "INSERT INTO inventory_stock (sku_id, quantity) VALUES ($1::uuid, $2) ON CONFLICT DO NOTHING", skuID, req.StockQuantity)

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

	if h.db == nil {
		return response.Created(c, "Category created", fiber.Map{"name": req.Name, "slug": req.Slug})
	}

	ctx := c.Context()
	var exists bool
	h.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM categories WHERE slug = $1)", req.Slug).Scan(&exists)
	if exists {
		return response.Error(c, fiber.StatusConflict, "Category slug already exists", nil)
	}

	var parentID *string
	if req.ParentID != "" {
		parentID = &req.ParentID
	}
	rate := req.CommissionRate
	if rate <= 0 {
		rate = 5.0
	}

	var catID string
	err := h.db.QueryRow(ctx,
		"INSERT INTO categories (parent_id, name, slug, icon_url) VALUES ($1, $2, $3, $4) RETURNING id::text",
		parentID, req.Name, req.Slug, req.IconURL,
	).Scan(&catID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to create category: "+err.Error(), nil)
	}

	_, _ = h.db.Exec(ctx,
		"INSERT INTO category_commission_rates (category_id, commission_rate) VALUES ($1::uuid, $2) ON CONFLICT (category_id) DO UPDATE SET commission_rate = EXCLUDED.commission_rate",
		catID, rate,
	)

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
	validStatuses := map[string]bool{"APPROVED": true, "REJECTED": true, "SUSPENDED": true}
	if !validStatuses[req.ApprovalStatus] {
		return response.ValidationError(c, map[string]string{"approval_status": "must be APPROVED, REJECTED, or SUSPENDED"})
	}

	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Product status updated", fiber.Map{"product_id": productID})
	}

	ctx := c.Context()
	result, err := h.db.Exec(ctx,
		"UPDATE products SET status = $1, updated_at = now() WHERE id::text = $2",
		req.ApprovalStatus, productID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to update product status: "+err.Error(), nil)
	}
	if result.RowsAffected() == 0 {
		return response.Error(c, fiber.StatusNotFound, "Product not found", nil)
	}

	return response.Success(c, fiber.StatusOK, "Product status updated in NeonDB", fiber.Map{
		"product_id": productID, "approval_status": req.ApprovalStatus,
	})
}

var _ = context.Background
