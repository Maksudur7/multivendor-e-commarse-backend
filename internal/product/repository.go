package product

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

type ProductSummary struct {
	ID              string    `json:"id"`
	ProductID       string    `json:"product_id"`
	Title           string    `json:"title"`
	Slug            string    `json:"slug"`
	PrimaryImageURL string    `json:"primary_image_url"`
	WholesalePrice  float64   `json:"wholesale_price"`
	RetailPrice     float64   `json:"retail_price"`
	IsResellable    bool      `json:"is_resellable"`
	Category        string    `json:"category"`
	Vendor          string    `json:"vendor"`
	CreatedAt       time.Time `json:"created_at"`
}

func (r *Repository) ListProducts(ctx context.Context, search, categorySlug string, limit, offset int) ([]ProductSummary, int, error) {
	if r.db == nil {
		return []ProductSummary{}, 0, nil
	}
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

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var products []ProductSummary
	for rows.Next() {
		var id, title, slug, catName, vendorName string
		var createdAt time.Time
		if err := rows.Scan(&id, &title, &slug, &catName, &vendorName, &createdAt); err == nil {
			products = append(products, ProductSummary{
				ID: id, ProductID: id, Title: title, Slug: slug, PrimaryImageURL: "",
				WholesalePrice: 800.0, RetailPrice: 1200.0, IsResellable: true,
				Category: catName, Vendor: vendorName, CreatedAt: createdAt,
			})
		}
	}
	if products == nil { products = []ProductSummary{} }

	var total int
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM products p LEFT JOIN categories c ON p.category_id = c.id %s`, whereClause)
	_ = r.db.QueryRow(ctx, countQuery, countArgs...).Scan(&total)

	return products, total, nil
}

type ProductDetail struct {
	ID               string                   `json:"id"`
	ProductID        string                   `json:"product_id"`
	Title            string                   `json:"title"`
	Slug             string                   `json:"slug"`
	Description      string                   `json:"description"`
	ShortDescription string                   `json:"short_description"`
	PrimaryImageURL  string                   `json:"primary_image_url"`
	WholesalePrice   float64                  `json:"wholesale_price"`
	RetailPrice      float64                  `json:"retail_price"`
	IsResellable     bool                     `json:"is_resellable"`
	IsFirstParty     bool                     `json:"is_first_party"`
	ApprovalStatus   string                   `json:"approval_status"`
	Category         string                   `json:"category"`
	Vendor           string                   `json:"vendor"`
	SKUs             []map[string]interface{} `json:"skus"`
}

func (r *Repository) GetProductDetailBySlug(ctx context.Context, slug string) (*ProductDetail, error) {
	if r.db == nil {
		return nil, nil
	}
	var id, title, description, shortDesc, approvalStatus, catName, vendorName string
	err := r.db.QueryRow(ctx, `
		SELECT p.id::text, p.name, COALESCE(p.description,''), COALESCE(p.short_description,''),
		       p.status, COALESCE(c.name,'') as cat, COALESCE(s.shop_name, u.full_name, 'First Party') as vendor
		FROM products p
		LEFT JOIN categories c ON p.category_id = c.id
		LEFT JOIN seller_profiles s ON p.seller_id = s.user_id
		LEFT JOIN users u ON p.seller_id = u.id
		WHERE p.slug = $1 AND p.deleted_at IS NULL`, slug).
		Scan(&id, &title, &description, &shortDesc, &approvalStatus, &catName, &vendorName)
	if err != nil {
		return nil, err
	}

	_, _ = r.db.Exec(ctx, "UPDATE products SET view_count = view_count + 1 WHERE slug = $1", slug)

	skuRows, _ := r.db.Query(ctx, `
		SELECT id::text, sku_code, wholesale_price, retail_price, stock_quantity
		FROM product_skus WHERE product_id = $1::uuid`, id)
	skus := []map[string]interface{}{}
	if skuRows != nil {
		defer skuRows.Close()
		for skuRows.Next() {
			var skuID, skuCode string
			var wPrice, rPrice float64
			var stock int
			if err := skuRows.Scan(&skuID, &skuCode, &wPrice, &rPrice, &stock); err == nil {
				skus = append(skus, map[string]interface{}{
					"id": skuID, "sku_code": skuCode,
					"wholesale_price": wPrice, "retail_price": rPrice, "stock_quantity": stock,
				})
			}
		}
	}

	return &ProductDetail{
		ID: id, ProductID: id, Title: title, Slug: slug, Description: description,
		ShortDescription: shortDesc, PrimaryImageURL: "", WholesalePrice: 800.0, RetailPrice: 1200.0,
		IsResellable: true, IsFirstParty: false, ApprovalStatus: approvalStatus,
		Category: catName, Vendor: vendorName, SKUs: skus,
	}, nil
}

type CategoryItem struct {
	ID             string  `json:"id"`
	CategoryID     string  `json:"category_id"`
	ParentID       string  `json:"parent_id"`
	Name           string  `json:"name"`
	Slug           string  `json:"slug"`
	IconURL        string  `json:"icon_url"`
	CommissionRate float64 `json:"commission_rate"`
}

func (r *Repository) GetCategoryTree(ctx context.Context) ([]CategoryItem, error) {
	if r.db == nil {
		return []CategoryItem{}, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT c.id::text, COALESCE(c.parent_id::text,'') as parent_id, c.name, c.slug,
		       COALESCE(c.icon_url,''), COALESCE(r.commission_rate, 10.0) as commission_rate
		FROM categories c
		LEFT JOIN category_commission_rates r ON c.id = r.category_id
		WHERE c.is_active = true ORDER BY c.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []CategoryItem
	for rows.Next() {
		var id, parentID, name, slug, iconURL string
		var rate float64
		if err := rows.Scan(&id, &parentID, &name, &slug, &iconURL, &rate); err == nil {
			categories = append(categories, CategoryItem{
				ID: id, CategoryID: id, ParentID: parentID, Name: name, Slug: slug,
				IconURL: iconURL, CommissionRate: rate,
			})
		}
	}
	if categories == nil { categories = []CategoryItem{} }
	return categories, nil
}

func (r *Repository) CreateProduct(ctx context.Context, userID, categoryID, title, slug, desc, shortDesc string) (string, error) {
	if r.db == nil {
		return "", nil
	}
	var catExists bool
	_ = r.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM categories WHERE id::text = $1)", categoryID).Scan(&catExists)
	if !catExists {
		return "", fmt.Errorf("category_not_found")
	}

	var slugExists bool
	_ = r.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM products WHERE slug = $1)", slug).Scan(&slugExists)
	if slugExists {
		return "", fmt.Errorf("slug_exists")
	}

	var productID string
	err := r.db.QueryRow(ctx, `
		INSERT INTO products (seller_id, category_id, name, slug, description, short_description, status)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, 'APPROVED') RETURNING id::text`,
		userID, categoryID, title, slug, desc, shortDesc,
	).Scan(&productID)
	return productID, err
}

func (r *Repository) AddSKU(ctx context.Context, productID, skuCode string, wholesalePrice, retailPrice float64, stock int) (string, error) {
	if r.db == nil {
		return "", nil
	}
	var productExists bool
	_ = r.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM products WHERE id::text = $1)", productID).Scan(&productExists)
	if !productExists {
		return "", fmt.Errorf("product_not_found")
	}

	var skuID string
	err := r.db.QueryRow(ctx, `
		INSERT INTO product_skus (product_id, sku_code, wholesale_price, retail_price, stock_quantity)
		VALUES ($1::uuid, $2, $3, $4, $5) RETURNING id::text`,
		productID, skuCode, wholesalePrice, retailPrice, stock,
	).Scan(&skuID)
	if err != nil {
		return "", err
	}

	_, _ = r.db.Exec(ctx, "INSERT INTO inventory_stock (sku_id, quantity) VALUES ($1::uuid, $2) ON CONFLICT DO NOTHING", skuID, stock)
	return skuID, nil
}

func (r *Repository) CreateCategory(ctx context.Context, name, slug, parentIDStr, iconURL string, rate float64) (string, error) {
	if r.db == nil {
		return "", nil
	}
	var exists bool
	_ = r.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM categories WHERE slug = $1)", slug).Scan(&exists)
	if exists {
		return "", fmt.Errorf("slug_exists")
	}

	var parentID *string
	if parentIDStr != "" { parentID = &parentIDStr }
	if rate <= 0 { rate = 5.0 }

	var catID string
	err := r.db.QueryRow(ctx,
		"INSERT INTO categories (parent_id, name, slug, icon_url) VALUES ($1, $2, $3, $4) RETURNING id::text",
		parentID, name, slug, iconURL,
	).Scan(&catID)
	if err != nil {
		return "", err
	}

	_, _ = r.db.Exec(ctx,
		"INSERT INTO category_commission_rates (category_id, commission_rate) VALUES ($1::uuid, $2) ON CONFLICT (category_id) DO UPDATE SET commission_rate = EXCLUDED.commission_rate",
		catID, rate,
	)
	return catID, nil
}

func (r *Repository) ApproveProduct(ctx context.Context, productID, status string) (int64, error) {
	if r.db == nil {
		return 0, nil
	}
	res, err := r.db.Exec(ctx, "UPDATE products SET status = $1, updated_at = now() WHERE id::text = $2", status, productID)
	if err != nil { return 0, err }
	return res.RowsAffected(), nil
}
