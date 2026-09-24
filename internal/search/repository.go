package search

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

type SearchHit struct {
	ProductID string  `json:"product_id"`
	Title     string  `json:"title"`
	Slug      string  `json:"slug"`
	Price     float64 `json:"price"`
}

func (r *Repository) SearchProducts(ctx context.Context, q string) ([]SearchHit, error) {
	if r.db == nil || q == "" {
		return []SearchHit{}, nil
	}
	queryPattern := "%" + q + "%"
	rows, err := r.db.Query(ctx, `
		SELECT id::text, COALESCE(name, title, ''), slug, COALESCE(price,0), is_active
		FROM products
		WHERE (name ILIKE $1 OR title ILIKE $1 OR description ILIKE $1) AND is_active = true
		LIMIT 50`, queryPattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hits []SearchHit
	for rows.Next() {
		var hit SearchHit
		var active bool
		if err := rows.Scan(&hit.ProductID, &hit.Title, &hit.Slug, &hit.Price, &active); err == nil {
			hits = append(hits, hit)
		}
	}
	if hits == nil { hits = []SearchHit{} }
	return hits, nil
}

func (r *Repository) GetSuggestions(ctx context.Context, q string) ([]string, error) {
	if r.db == nil || q == "" {
		return []string{q + " pro", q + " wireless", q + " original"}, nil
	}
	queryPattern := "%" + q + "%"
	rows, err := r.db.Query(ctx, "SELECT COALESCE(name, title, '') FROM products WHERE name ILIKE $1 OR title ILIKE $1 LIMIT 5", queryPattern)
	if err != nil {
		return []string{q + " pro", q + " wireless", q + " original"}, nil
	}
	defer rows.Close()

	var suggestions []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err == nil && t != "" {
			suggestions = append(suggestions, t)
		}
	}
	if len(suggestions) == 0 {
		suggestions = []string{q + " pro", q + " wireless", q + " original"}
	}
	return suggestions, nil
}
