package adminfraud

import (
	"time"

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
	fraud := router.Group("/admin/fraud", authMiddleware)

	// 8 Admin Fraud Endpoints
	fraud.Get("/risk-profiles", h.ListRiskProfiles)
	fraud.Get("/blacklists", h.ListBlacklists)
	fraud.Get("/ip-blocks", h.ListIPBlocks)
	fraud.Post("/blacklist-phone", h.BlacklistPhone)
	fraud.Post("/block-ip", h.BlockIP)
	fraud.Post("/recalculate-risk", h.RecalculateRisk)
	fraud.Put("/risk-profiles/:id", h.UpdateRiskProfile)
	fraud.Delete("/ip-blocks/:id", h.UnblockIP)
}

func (h *Handler) ListRiskProfiles(c *fiber.Ctx) error {
	riskLevel := c.Query("risk_level")
	profiles, err := h.service.ListRiskProfiles(c.Context(), riskLevel)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load risk profiles: "+err.Error(), nil)
	}
	return response.Success(c, fiber.StatusOK, "Risk profiles loaded", fiber.Map{
		"profiles": profiles, "count": len(profiles),
	})
}

func (h *Handler) ListBlacklists(c *fiber.Ctx) error {
	blacklists, err := h.service.ListBlacklists(c.Context())
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load blacklists: "+err.Error(), nil)
	}
	return response.Success(c, fiber.StatusOK, "Blacklisted phones loaded", fiber.Map{
		"blacklists": blacklists, "count": len(blacklists),
	})
}

func (h *Handler) ListIPBlocks(c *fiber.Ctx) error {
	ipBlocks, err := h.service.ListIPBlocks(c.Context())
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load IP blocks: "+err.Error(), nil)
	}
	return response.Success(c, fiber.StatusOK, "Active IP blocks loaded", fiber.Map{
		"ip_blocks": ipBlocks, "count": len(ipBlocks),
	})
}

type BlacklistPhoneReq struct {
	Phone  string `json:"phone"`
	Reason string `json:"reason"`
	Note   string `json:"note"`
}

func (h *Handler) BlacklistPhone(c *fiber.Ctx) error {
	var req BlacklistPhoneReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}
	if req.Phone == "" || req.Reason == "" {
		return response.ValidationError(c, map[string]string{"phone": "phone and reason are required"})
	}

	blacklistID, err := h.service.BlacklistPhone(c.Context(), req.Phone, req.Reason, req.Note)
	if err != nil {
		return response.ValidationError(c, map[string]string{"reason": err.Error()})
	}
	return response.Created(c, "Phone blacklisted successfully", fiber.Map{
		"blacklist_id": blacklistID, "phone": req.Phone, "reason": req.Reason,
	})
}

type BlockIPReq struct {
	IPAddress     string `json:"ip_address"`
	Reason        string `json:"reason"`
	DurationHours int    `json:"duration_hours"`
}

func (h *Handler) BlockIP(c *fiber.Ctx) error {
	var req BlockIPReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}

	blockID, blockedUntil, err := h.service.BlockIP(c.Context(), req.IPAddress, req.Reason, req.DurationHours)
	if err != nil {
		return response.ValidationError(c, map[string]string{"ip_address": err.Error()})
	}

	return response.Created(c, "IP address blocked successfully", fiber.Map{
		"block_id":       blockID,
		"ip_address":     req.IPAddress,
		"blocked_until":  blockedUntil.Format(time.RFC3339),
		"duration_hours": req.DurationHours,
	})
}

type RecalculateRiskReq struct {
	UserID string `json:"user_id"`
}

func (h *Handler) RecalculateRisk(c *fiber.Ctx) error {
	var req RecalculateRiskReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}
	if req.UserID == "" {
		return response.ValidationError(c, map[string]string{"user_id": "required"})
	}

	if err := h.service.RecalculateRisk(c.Context(), req.UserID); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	return response.Success(c, fiber.StatusOK, "Risk score recalculated for user", fiber.Map{
		"user_id": req.UserID, "status": "RECALCULATED",
	})
}

type UpdateRiskProfileReq struct {
	RiskLevel string  `json:"risk_level"`
	AdminNote string  `json:"admin_note"`
	RiskScore float64 `json:"risk_score"`
}

func (h *Handler) UpdateRiskProfile(c *fiber.Ctx) error {
	profileID := c.Params("id")
	var req UpdateRiskProfileReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}

	if err := h.service.UpdateRiskProfile(c.Context(), profileID, req.RiskLevel, req.AdminNote, req.RiskScore); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	return response.Success(c, fiber.StatusOK, "Risk profile updated", fiber.Map{
		"profile_id": profileID, "risk_level": req.RiskLevel, "manually_reviewed": true,
	})
}

func (h *Handler) UnblockIP(c *fiber.Ctx) error {
	blockID := c.Params("id")
	if err := h.service.UnblockIP(c.Context(), blockID); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, err.Error(), nil)
	}
	return response.Success(c, fiber.StatusOK, "IP unblocked", fiber.Map{"block_id": blockID})
}
