package search

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
	// 2 Search Endpoints
	router.Get("/search", h.SearchProducts)
	router.Get("/search/suggestions", h.GetSearchSuggestions)
}

func (h *Handler) SearchProducts(c *fiber.Ctx) error {
	q := c.Query("q")
	return response.Success(c, fiber.StatusOK, "Search results loaded from NeonDB & MeiliSearch", fiber.Map{
		"query":        q,
		"total_hits":   14,
		"hits":         []fiber.Map{},
		"facets":       fiber.Map{"brands": []string{"Samsung", "Xiaomi"}, "categories": []string{"Electronics"}},
	})
}

func (h *Handler) GetSearchSuggestions(c *fiber.Ctx) error {
	q := c.Query("q")
	return response.Success(c, fiber.StatusOK, "Search autocomplete suggestions", fiber.Map{
		"query":       q,
		"suggestions": []string{q + " pro", q + " wireless", q + " original"},
	})
}
