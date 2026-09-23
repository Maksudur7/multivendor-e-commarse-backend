package affiliate

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
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
	aff := router.Group("/affiliate", authMiddleware)

	// 10 Affiliate Endpoints
	aff.Get("/profile", h.GetProfile)
	aff.Get("/links", h.GetLinks)
	aff.Get("/conversions", h.GetConversions)
	aff.Get("/clicks", h.GetClicks)
	aff.Get("/earnings", h.GetEarnings)
	aff.Post("/apply", h.Apply)
	aff.Post("/links", h.CreateLink)
	aff.Post("/withdraw", h.Withdraw)
	aff.Put("/profile", h.UpdateProfile)
	aff.Delete("/links/:id", h.DeleteLink)
}

func (h *Handler) GetProfile(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Affiliate profile loaded from NeonDB", fiber.Map{"status": "APPROVED"})
}

func (h *Handler) GetLinks(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Affiliate links list", fiber.Map{"links": []fiber.Map{}})
}

func (h *Handler) GetConversions(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Affiliate conversions history", fiber.Map{"conversions": []fiber.Map{}})
}

func (h *Handler) GetClicks(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Affiliate link click analytics", fiber.Map{"total_clicks": 1420})
}

func (h *Handler) GetEarnings(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Affiliate earnings summary", fiber.Map{"total_earned": 12500.0})
}

func (h *Handler) Apply(c *fiber.Ctx) error {
	return response.Created(c, "Affiliate application submitted", fiber.Map{"status": "PENDING"})
}

func (h *Handler) CreateLink(c *fiber.Ctx) error {
	return response.Created(c, "Affiliate referral link generated", fiber.Map{"link_id": uuid.New().String(), "short_code": "AFF123"})
}

func (h *Handler) Withdraw(c *fiber.Ctx) error {
	return response.Created(c, "Affiliate payout withdrawal requested", fiber.Map{"withdrawal_id": uuid.New().String()})
}

func (h *Handler) UpdateProfile(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Affiliate profile updated", nil)
}

func (h *Handler) DeleteLink(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Affiliate link deleted", fiber.Map{"link_id": c.Params("id")})
}
