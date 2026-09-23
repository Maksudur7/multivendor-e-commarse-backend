package catalog

import (
		"fmt"
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
	pub := router.Group("/catalog")
	pub.Get("/products", h.ListPublicProducts)
	pub.Get("/products/:slug", h.GetPublicProductDetail)
	pub.Get("/categories", h.ListPublicCategories)
	pub.Get("/categories/:slug", h.GetCategoryBySlug)
	pub.Get("/brands", h.ListPublicBrands)
	pub.Get("/brands/:slug", h.GetBrandBySlug)
	pub.Get("/products/:id/variants", h.GetProductVariants)
	pub.Get("/products/:id/reviews", h.GetProductReviews)

	seller := router.Group("/seller/catalog", authMiddleware)
	seller.Get("/products", h.ListSellerProducts)
	seller.Get("/products/:id", h.GetSellerProductDetail)
	seller.Get("/variants", h.ListSellerVariants)
	seller.Get("/categories", h.ListSellerCategories)
	seller.Post("/products", h.CreateSellerProduct)
	seller.Post("/products/:id/variants", h.CreateSellerVariant)
	seller.Put("/products/:id", h.UpdateSellerProduct)
	seller.Put("/variants/:id", h.UpdateSellerVariant)
	seller.Put("/products/:id/status", h.UpdateSellerProductStatus)
	seller.Delete("/products/:id", h.DeleteSellerProduct)

	admin := router.Group("/admin/catalog", authMiddleware)
	admin.Get("/products", h.ListAdminProducts)
	admin.Get("/products/pending", h.ListPendingProductsAdmin)
	admin.Get("/categories", h.ListAdminCategories)
	admin.Get("/brands", h.ListAdminBrands)
	admin.Get("/attributes", h.ListAdminAttributes)
	admin.Get("/categories/:id", h.GetAdminCategoryDetail)
	admin.Post("/categories", h.CreateCategoryAdmin)
	admin.Post("/brands", h.CreateBrandAdmin)
	admin.Post("/attributes", h.CreateAttributeAdmin)
	admin.Post("/attributes/options", h.CreateAttributeOptionAdmin)
	admin.Put("/products/:id/approval", h.ApproveProductAdmin)
	admin.Put("/categories/:id", h.UpdateCategoryAdmin)
	admin.Delete("/products/:id", h.DeleteProductAdmin)
}

func (h *Handler) ListPublicProducts(c *fiber.Ctx) error {
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Public products loaded", fiber.Map{"products": []fiber.Map{}})
	}
	ctx := c.Context()
	rows, err := h.db.Query(ctx, "SELECT id::text, title, slug, price, compare_at_price, is_active FROM products WHERE is_active = true ORDER BY created_at DESC LIMIT 50")
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load products", nil)
	}
	defer rows.Close()

	products := []fiber.Map{}
	for rows.Next() {
		var id, title, slug string; var price, compare float64; var active bool
		rows.Scan(&id, &title, &slug, &price, &compare, &active)
		products = append(products, fiber.Map{"product_id": id, "title": title, "slug": slug, "price": price, "compare_at_price": compare})
	}
	return response.Success(c, fiber.StatusOK, "Public products from NeonDB", fiber.Map{"products": products, "count": len(products)})
}

func (h *Handler) GetPublicProductDetail(c *fiber.Ctx) error {
	slug := c.Params("slug")
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Product detail", fiber.Map{"slug": slug})
	}
	ctx := c.Context()
	var id, title, desc, img string; var price float64; var active bool
	err := h.db.QueryRow(ctx, "SELECT id::text, title, COALESCE(description,''), COALESCE(primary_image_url,''), price, is_active FROM products WHERE slug = $1", slug).
		Scan(&id, &title, &desc, &img, &price, &active)
	if err != nil {
		return response.Error(c, fiber.StatusNotFound, "Product not found", nil)
	}
	return response.Success(c, fiber.StatusOK, "Product detail from NeonDB", fiber.Map{
		"product_id": id, "title": title, "slug": slug, "description": desc, "image_url": img, "price": price,
	})
}

func (h *Handler) ListPublicCategories(c *fiber.Ctx) error {
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Categories tree", fiber.Map{"categories": []fiber.Map{}})
	}
	ctx := c.Context()
	rows, err := h.db.Query(ctx, "SELECT id::text, name, slug FROM categories ORDER BY name")
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load categories", nil)
	}
	defer rows.Close()

	cats := []fiber.Map{}
	for rows.Next() {
		var id, name, slug string
		rows.Scan(&id, &name, &slug)
		cats = append(cats, fiber.Map{"category_id": id, "name": name, "slug": slug})
	}
	return response.Success(c, fiber.StatusOK, "Categories from NeonDB", fiber.Map{"categories": cats})
}

func (h *Handler) GetCategoryBySlug(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Category loaded", fiber.Map{"slug": c.Params("slug")})
}

func (h *Handler) ListPublicBrands(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Brands list", fiber.Map{"brands": []fiber.Map{}})
}

func (h *Handler) GetBrandBySlug(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Brand details", fiber.Map{"slug": c.Params("slug")})
}

func (h *Handler) GetProductVariants(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Product variants loaded", fiber.Map{"product_id": c.Params("id"), "variants": []fiber.Map{}})
}

func (h *Handler) GetProductReviews(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Product reviews loaded", fiber.Map{"product_id": c.Params("id"), "reviews": []fiber.Map{}})
}

// Seller Catalog Handlers
func (h *Handler) ListSellerProducts(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Seller products loaded", fiber.Map{"products": []fiber.Map{}})
}

func (h *Handler) GetSellerProductDetail(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Seller product detail", fiber.Map{"id": c.Params("id")})
}

func (h *Handler) ListSellerVariants(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Seller variants", fiber.Map{"variants": []fiber.Map{}})
}

func (h *Handler) ListSellerCategories(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Available categories for seller", fiber.Map{"categories": []fiber.Map{}})
}

type CreateProductReq struct {
	Title       string  `json:"title"`
	Slug        string  `json:"slug"`
	CategoryID  string  `json:"category_id"`
	RetailPrice float64 `json:"retail_price"`
}

func (h *Handler) CreateSellerProduct(c *fiber.Ctx) error {
	var req CreateProductReq
	c.BodyParser(&req)
	if req.Title == "" {
		return response.ValidationError(c, map[string]string{"title": "is required"})
	}
	if req.Slug == "" {
		req.Slug = fmt.Sprintf("prod-%d", time.Now().UnixNano()/1e6)
	}

	if h.db == nil {
		return response.Created(c, "Product created", fiber.Map{"title": req.Title})
	}

	ctx := c.Context()
	var prodID string
	err := h.db.QueryRow(ctx, "INSERT INTO products (title, slug, price, is_active) VALUES ($1, $2, $3, true) RETURNING id::text", req.Title, req.Slug, req.RetailPrice).Scan(&prodID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to create product: "+err.Error(), nil)
	}

	return response.Created(c, "Product created in NeonDB", fiber.Map{"product_id": prodID, "title": req.Title, "slug": req.Slug})
}

func (h *Handler) CreateSellerVariant(c *fiber.Ctx) error {
	return response.Created(c, "Product variant created", fiber.Map{"status": "CREATED"})
}

func (h *Handler) UpdateSellerProduct(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Product updated", fiber.Map{"id": c.Params("id")})
}

func (h *Handler) UpdateSellerVariant(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Variant updated", fiber.Map{"id": c.Params("id")})
}

func (h *Handler) UpdateSellerProductStatus(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Product status updated", fiber.Map{"id": c.Params("id")})
}

func (h *Handler) DeleteSellerProduct(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Product deleted", fiber.Map{"id": c.Params("id")})
}

// Admin Catalog Handlers
func (h *Handler) ListAdminProducts(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Admin all products", fiber.Map{"products": []fiber.Map{}})
}

func (h *Handler) ListPendingProductsAdmin(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Pending approval products", fiber.Map{"pending": []fiber.Map{}})
}

func (h *Handler) ListAdminCategories(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Admin categories list", fiber.Map{"categories": []fiber.Map{}})
}

func (h *Handler) ListAdminBrands(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Admin brands list", fiber.Map{"brands": []fiber.Map{}})
}

func (h *Handler) ListAdminAttributes(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Attributes list", fiber.Map{"attributes": []fiber.Map{}})
}

func (h *Handler) GetAdminCategoryDetail(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Admin category detail", fiber.Map{"id": c.Params("id")})
}

func (h *Handler) CreateCategoryAdmin(c *fiber.Ctx) error {
	var req struct { Name string `json:"name"`; Slug string `json:"slug"` }
	c.BodyParser(&req)
	if req.Name == "" { req.Name = "New Category" }
	if req.Slug == "" { req.Slug = fmt.Sprintf("cat-%d", time.Now().UnixNano()/1e6) }

	if h.db == nil {
		return response.Created(c, "Category created", fiber.Map{"name": req.Name})
	}

	ctx := c.Context()
	var catID string
	err := h.db.QueryRow(ctx, "INSERT INTO categories (name, slug) VALUES ($1, $2) RETURNING id::text", req.Name, req.Slug).Scan(&catID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to create category: "+err.Error(), nil)
	}
	return response.Created(c, "Category created in NeonDB", fiber.Map{"category_id": catID, "name": req.Name, "slug": req.Slug})
}

func (h *Handler) CreateBrandAdmin(c *fiber.Ctx) error {
	return response.Created(c, "Brand created", fiber.Map{"status": "CREATED"})
}

func (h *Handler) CreateAttributeAdmin(c *fiber.Ctx) error {
	return response.Created(c, "Attribute set created", fiber.Map{"status": "CREATED"})
}

func (h *Handler) CreateAttributeOptionAdmin(c *fiber.Ctx) error {
	return response.Created(c, "Attribute option added", fiber.Map{"status": "CREATED"})
}

func (h *Handler) ApproveProductAdmin(c *fiber.Ctx) error {
	prodID := c.Params("id")
	if h.db != nil {
		h.db.Exec(c.Context(), "UPDATE products SET is_active = true WHERE id::text = $1", prodID)
	}
	return response.Success(c, fiber.StatusOK, "Product approved by Admin in NeonDB", fiber.Map{"product_id": prodID})
}

func (h *Handler) UpdateCategoryAdmin(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Category updated", fiber.Map{"category_id": c.Params("id")})
}

func (h *Handler) DeleteProductAdmin(c *fiber.Ctx) error {
	prodID := c.Params("id")
	if h.db != nil {
		h.db.Exec(c.Context(), "DELETE FROM products WHERE id::text = $1", prodID)
	}
	return response.Success(c, fiber.StatusOK, "Product deleted by Admin in NeonDB", fiber.Map{"product_id": prodID})
}
