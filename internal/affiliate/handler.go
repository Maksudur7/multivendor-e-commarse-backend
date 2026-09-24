package affiliate

import (
	"fmt"

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
	userID := c.Locals("user_id").(string)
	profile, err := h.service.GetProfile(c.Context(), userID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load affiliate profile", nil)
	}
	return response.Success(c, fiber.StatusOK, "Affiliate profile loaded", profile)
}

func (h *Handler) GetLinks(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	links, err := h.service.GetLinks(c.Context(), userID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load links: "+err.Error(), nil)
	}
	return response.Success(c, fiber.StatusOK, "Affiliate links loaded", fiber.Map{
		"links": links, "count": len(links),
	})
}

func (h *Handler) GetConversions(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Affiliate conversions", fiber.Map{
		"conversions": []fiber.Map{}, "count": 0,
	})
}

func (h *Handler) GetClicks(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	totalClicks, err := h.service.GetClicks(c.Context(), userID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to get click stats", nil)
	}
	return response.Success(c, fiber.StatusOK, "Affiliate click stats", fiber.Map{
		"total_clicks": totalClicks,
		"user_id":      userID,
		"period":       "all_time",
	})
}

func (h *Handler) GetEarnings(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	totalEarned, err := h.service.GetEarnings(c.Context(), userID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to get earnings", nil)
	}
	return response.Success(c, fiber.StatusOK, "Affiliate earnings", fiber.Map{
		"total_earned": totalEarned,
		"user_id":      userID,
		"currency":     "BDT",
	})
}

type ApplyReq struct {
	WebsiteURL      string `json:"website_url"`
	SocialLinks     string `json:"social_links"`
	PromotionMethod string `json:"promotion_method"`
}

func (h *Handler) Apply(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	var req ApplyReq
	_ = c.BodyParser(&req)

	profileID, refCode, err := h.service.Apply(c.Context(), userID)
	if err != nil {
		return response.Error(c, fiber.StatusConflict, err.Error(), nil)
	}

	return response.Created(c, "Affiliate application submitted", fiber.Map{
		"affiliate_id":  profileID,
		"status":        "PENDING",
		"referral_code": refCode,
		"user_id":       userID,
	})
}

type CreateLinkReq struct {
	ProductID string `json:"product_id"`
	FullURL   string `json:"full_url"`
	Label     string `json:"label"`
}

func (h *Handler) CreateLink(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	var req CreateLinkReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}
	if req.FullURL == "" {
		return response.ValidationError(c, map[string]string{"full_url": "required"})
	}

	linkID, shortCode, err := h.service.CreateLink(c.Context(), userID, req.ProductID, req.FullURL)
	if err != nil {
		return response.Error(c, fiber.StatusForbidden, err.Error(), nil)
	}

	return response.Created(c, "Affiliate referral link generated", fiber.Map{
		"link_id":    linkID,
		"short_code": shortCode,
		"full_url":   req.FullURL,
		"share_url":  fmt.Sprintf("https://platform.com/r/%s", shortCode),
	})
}

type WithdrawReq struct {
	Amount        float64 `json:"amount"`
	PaymentMethod string  `json:"payment_method"`
	AccountNumber string  `json:"account_number"`
}

func (h *Handler) Withdraw(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	var req WithdrawReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}
	if req.Amount <= 0 {
		return response.ValidationError(c, map[string]string{"amount": "must be greater than 0"})
	}

	withdrawalID, err := h.service.Withdraw(c.Context(), userID, req.Amount)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error(), nil)
	}

	return response.Created(c, "Affiliate withdrawal requested", fiber.Map{
		"withdrawal_id":  withdrawalID,
		"amount":         req.Amount,
		"payment_method": req.PaymentMethod,
		"status":         "PENDING",
	})
}

func (h *Handler) UpdateProfile(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	_ = h.service.UpdateProfile(c.Context(), userID)
	return response.Success(c, fiber.StatusOK, "Affiliate profile updated", fiber.Map{"user_id": userID})
}

func (h *Handler) DeleteLink(c *fiber.Ctx) error {
	linkID := c.Params("id")
	userID := c.Locals("user_id").(string)
	if err := h.service.DeleteLink(c.Context(), linkID, userID); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to delete link", nil)
	}
	return response.Success(c, fiber.StatusOK, "Affiliate link deleted", fiber.Map{
		"link_id": linkID,
	})
}
