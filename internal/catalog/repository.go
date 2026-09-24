package catalog

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

type PublicProductItem struct {
	ProductID      string  `json:"product_id"`
	Title          string  `json:"title"`
	Slug           string  `json:"slug"`
	Price          float64 `json:"price"`
	CompareAtPrice float64 `json:"compare_at_price"`
}

func (r *Repository) ListPublicProducts(ctx context.Context) ([]PublicProductItem, error) {
	if r.db == nil {
		return []PublicProductItem{}, nil
	}
	rows, err := r.db.Query(ctx, "SELECT id::text, title, slug, price, compare_at_price, is_active FROM products WHERE is_active = true ORDER BY created_at DESC LIMIT 50")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []PublicProductItem
	for rows.Next() {
		var item PublicProductItem
		var active bool
		if err := rows.Scan(&item.ProductID, &item.Title, &item.Slug, &item.Price, &item.CompareAtPrice, &active); err == nil {
			list = append(list, item)
		}
	}
	if list == nil { list = []PublicProductItem{} }
	return list, nil
}

type ProductDetail struct {
	ProductID   string  `json:"product_id"`
	Title       string  `json:"title"`
	Slug        string  `json:"slug"`
	Description string  `json:"description"`
	ImageURL    string  `json:"image_url"`
	Price       float64 `json:"price"`
}

func (r *Repository) GetPublicProductBySlug(ctx context.Context, slug string) (*ProductDetail, error) {
	if r.db == nil {
		return &ProductDetail{Slug: slug, Title: "Sample Product", Price: 1000.0}, nil
	}
	var p ProductDetail
	p.Slug = slug
	var active bool
	err := r.db.QueryRow(ctx, "SELECT id::text, title, COALESCE(description,''), COALESCE(primary_image_url,''), price, is_active FROM products WHERE slug = $1", slug).
		Scan(&p.ProductID, &p.Title, &p.Description, &p.ImageURL, &p.Price, &active)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

type CategoryItem struct {
	CategoryID string `json:"category_id"`
	Name       string `json:"name"`
	Slug       string `json:"slug"`
}

func (r *Repository) ListPublicCategories(ctx context.Context) ([]CategoryItem, error) {
	if r.db == nil {
		return []CategoryItem{}, nil
	}
	rows, err := r.db.Query(ctx, "SELECT id::text, name, slug FROM categories ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []CategoryItem
	for rows.Next() {
		var item CategoryItem
		if err := rows.Scan(&item.CategoryID, &item.Name, &item.Slug); err == nil {
			list = append(list, item)
		}
	}
	if list == nil { list = []CategoryItem{} }
	return list, nil
}

func (r *Repository) CreateSellerProduct(ctx context.Context, title, slug string, price float64) (string, error) {
	if r.db == nil {
		return "mock-prod-id", nil
	}
	var id string
	err := r.db.QueryRow(ctx, "INSERT INTO products (title, slug, price, is_active) VALUES ($1, $2, $3, true) RETURNING id::text", title, slug, price).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (r *Repository) CreateCategory(ctx context.Context, name, slug string) (string, error) {
	if r.db == nil {
		return "mock-cat-id", nil
	}
	var id string
	err := r.db.QueryRow(ctx, "INSERT INTO categories (name, slug) VALUES ($1, $2) RETURNING id::text", name, slug).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}
