package adminfinance

import (
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
	fin := router.Group("/admin/finance", authMiddleware)

	// 11 Admin Finance Endpoints
	fin.Get("/overview", h.GetFinancialOverview)
	fin.Get("/payouts", h.ListPayoutRequests)
	fin.Get("/escrow", h.ListEscrowHoldings)
	fin.Get("/commissions", h.ListCommissionsEarned)
	fin.Get("/reports", h.GetFinancialReports)
	fin.Post("/payouts/approve", h.ApprovePayout)
	fin.Post("/payouts/process", h.ProcessPayout)
	fin.Post("/escrow/release", h.ReleaseEscrowManual)
	fin.Post("/manual-credit", h.ManualCreditWallet)
	fin.Put("/payouts/:id", h.UpdatePayoutStatus)
	fin.Put("/commission-rates", h.UpdateCommissionRates)
}

func (h *Handler) GetFinancialOverview(c *fiber.Ctx) error {
	overview, err := h.service.GetOverview(c.Context())
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load overview", nil)
	}
	return response.Success(c, fiber.StatusOK, "Financial dashboard metrics", overview)
}

func (h *Handler) ListPayoutRequests(c *fiber.Ctx) error {
	status := c.Query("status")
	payouts, err := h.service.ListPayoutRequests(c.Context(), status)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load payout requests: "+err.Error(), nil)
	}
	return response.Success(c, fiber.StatusOK, "Payout requests", fiber.Map{
		"payouts": payouts, "count": len(payouts),
	})
}

func (h *Handler) ListEscrowHoldings(c *fiber.Ctx) error {
	escrow, err := h.service.ListEscrowHoldings(c.Context())
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load escrow data: "+err.Error(), nil)
	}
	return response.Success(c, fiber.StatusOK, "Escrow holdings", fiber.Map{
		"escrow": escrow, "count": len(escrow),
	})
}

func (h *Handler) ListCommissionsEarned(c *fiber.Ctx) error {
	commissions, err := h.service.ListCommissionsEarned(c.Context())
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load commissions", nil)
	}
	return response.Success(c, fiber.StatusOK, "Platform commissions", fiber.Map{
		"commissions": commissions, "count": len(commissions),
	})
}

func (h *Handler) GetFinancialReports(c *fiber.Ctx) error {
	reports, err := h.service.GetFinancialReports(c.Context())
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load reports: "+err.Error(), nil)
	}
	return response.Success(c, fiber.StatusOK, "Financial reports", fiber.Map{
		"reports": reports, "count": len(reports),
	})
}

type ApprovePayoutReq struct {
	PayoutID string `json:"payout_id"`
	Note     string `json:"note"`
}

func (h *Handler) ApprovePayout(c *fiber.Ctx) error {
	var req ApprovePayoutReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}
	if req.PayoutID == "" {
		return response.ValidationError(c, map[string]string{"payout_id": "required"})
	}
	if err := h.service.ApprovePayout(c.Context(), req.PayoutID); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, err.Error(), nil)
	}
	return response.Success(c, fiber.StatusOK, "Payout request approved", fiber.Map{
		"payout_id": req.PayoutID, "status": "APPROVED",
	})
}

type ProcessPayoutReq struct {
	PayoutID            string `json:"payout_id"`
	TransactionProofRef string `json:"transaction_proof_ref"`
}

func (h *Handler) ProcessPayout(c *fiber.Ctx) error {
	var req ProcessPayoutReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}
	if req.PayoutID == "" {
		return response.ValidationError(c, map[string]string{"payout_id": "required"})
	}
	if err := h.service.ProcessPayout(c.Context(), req.PayoutID, req.TransactionProofRef); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, err.Error(), nil)
	}
	return response.Success(c, fiber.StatusOK, "Payout processed", fiber.Map{
		"payout_id": req.PayoutID, "status": "COMPLETED", "proof_ref": req.TransactionProofRef,
	})
}

type ReleaseEscrowReq struct {
	UserID string  `json:"user_id"`
	Amount float64 `json:"amount"`
	Note   string  `json:"note"`
}

func (h *Handler) ReleaseEscrowManual(c *fiber.Ctx) error {
	var req ReleaseEscrowReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}
	if req.UserID == "" || req.Amount <= 0 {
		return response.ValidationError(c, map[string]string{"escrow": "user_id and amount > 0 required"})
	}
	if err := h.service.ReleaseEscrow(c.Context(), req.UserID, req.Amount); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, err.Error(), nil)
	}
	return response.Success(c, fiber.StatusOK, "Escrow released to vendor wallet", fiber.Map{
		"user_id": req.UserID, "released_amount": req.Amount,
	})
}

type ManualCreditReq struct {
	UserID      string  `json:"user_id"`
	Amount      float64 `json:"amount"`
	Type        string  `json:"type"`
	Description string  `json:"description"`
}

func (h *Handler) ManualCreditWallet(c *fiber.Ctx) error {
	var req ManualCreditReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}
	if req.UserID == "" || req.Amount <= 0 {
		return response.ValidationError(c, map[string]string{"credit": "user_id and amount > 0 required"})
	}

	if err := h.service.ManualCreditWallet(c.Context(), req.UserID, req.Type, req.Amount); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, err.Error(), nil)
	}
	return response.Created(c, "Manual credit/debit applied to user wallet", fiber.Map{
		"user_id": req.UserID, "amount": req.Amount, "type": req.Type, "description": req.Description,
	})
}

func (h *Handler) UpdatePayoutStatus(c *fiber.Ctx) error {
	payoutID := c.Params("id")
	var body struct {
		Status string `json:"status"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}
	if body.Status == "" {
		return response.ValidationError(c, map[string]string{"status": "required"})
	}
	if err := h.service.UpdatePayoutStatus(c.Context(), payoutID, body.Status); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, err.Error(), nil)
	}
	return response.Success(c, fiber.StatusOK, "Payout status updated", fiber.Map{
		"id": payoutID, "status": body.Status,
	})
}

type CommissionRateReq struct {
	CategoryID     string  `json:"category_id"`
	CommissionRate float64 `json:"commission_rate"`
}

func (h *Handler) UpdateCommissionRates(c *fiber.Ctx) error {
	var req CommissionRateReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}
	if req.CommissionRate <= 0 || req.CommissionRate > 50 {
		return response.ValidationError(c, map[string]string{"commission_rate": "must be between 0 and 50"})
	}

	if err := h.service.UpdateCommissionRates(c.Context(), req.CategoryID, req.CommissionRate); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, err.Error(), nil)
	}
	return response.Success(c, fiber.StatusOK, "Commission rates updated", fiber.Map{
		"category_id": req.CategoryID, "commission_rate": req.CommissionRate,
	})
}
