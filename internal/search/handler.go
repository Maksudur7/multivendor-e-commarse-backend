package search

import (
	"strings"

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
	router.Get("/search", h.SearchProducts)
	router.Get("/search/suggestions", h.GetSearchSuggestions)
}

func (h *Handler) SearchProducts(c *fiber.Ctx) error {
	q := strings.TrimSpace(c.Query("q"))
	if h.db == nil || q == "" {
		return response.Success(c, fiber.StatusOK, "Search results", fiber.Map{
			"query": q, "total_hits": 0, "hits": []fiber.Map{},
		})
	}

	ctx := c.Context()
	queryPattern := "%" + q + "%"
	rows, err := h.db.Query(ctx, `
		SELECT id::text, COALESCE(name, title, ''), slug, COALESCE(price,0), is_active
		FROM products
		WHERE (name ILIKE $1 OR title ILIKE $1 OR description ILIKE $1) AND is_active = true
		LIMIT 50`, queryPattern)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Search query failed: "+err.Error(), nil)
	}
	defer rows.Close()

	hits := []fiber.Map{}
	for rows.Next() {
		var id, title, slug string
		var price float64
		var active bool
		rows.Scan(&id, &title, &slug, &price, &active)
		hits = append(hits, fiber.Map{
			"product_id": id, "title": title, "slug": slug, "price": price,
		})
	}

	return response.Success(c, fiber.StatusOK, "Search results loaded from NeonDB", fiber.Map{
		"query":      q,
		"total_hits": len(hits),
		"hits":       hits,
	})
}

func (h *Handler) GetSearchSuggestions(c *fiber.Ctx) error {
	q := strings.TrimSpace(c.Query("q"))
	if h.db == nil || q == "" {
		return response.Success(c, fiber.StatusOK, "Search autocomplete suggestions", fiber.Map{
			"query": q, "suggestions": []string{q + " pro", q + " wireless", q + " original"},
		})
	}

	ctx := c.Context()
	queryPattern := "%" + q + "%"
	rows, err := h.db.Query(ctx, "SELECT COALESCE(name, title, '') FROM products WHERE name ILIKE $1 OR title ILIKE $1 LIMIT 5", queryPattern)
	suggestions := []string{}
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var t string
			rows.Scan(&t)
			suggestions = append(suggestions, t)
		}
	}
	if len(suggestions) == 0 {
		suggestions = []string{q + " pro", q + " wireless", q + " original"}
	}

	return response.Success(c, fiber.StatusOK, "Search autocomplete suggestions from NeonDB", fiber.Map{
		"query":       q,
		"suggestions": suggestions,
	})
}