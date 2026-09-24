package search

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
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
	router.Get("/search", h.SearchProducts)
	router.Get("/search/suggestions", h.GetSearchSuggestions)
}

func (h *Handler) SearchProducts(c *fiber.Ctx) error {
	q := strings.TrimSpace(c.Query("q"))
	result, err := h.service.SearchProducts(c.Context(), q)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Search query failed: "+err.Error(), nil)
	}
	return response.Success(c, fiber.StatusOK, "Search results loaded", result)
}

func (h *Handler) GetSearchSuggestions(c *fiber.Ctx) error {
	q := strings.TrimSpace(c.Query("q"))
	result, err := h.service.GetSuggestions(c.Context(), q)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load suggestions", nil)
	}
	return response.Success(c, fiber.StatusOK, "Search autocomplete suggestions", result)
}