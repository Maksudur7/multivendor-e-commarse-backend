package wallet

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
	w := router.Group("/wallet", authMiddleware)
	w.Get("/", h.GetWalletBalance)
	w.Get("/transactions", h.GetLedgerHistory)
	w.Post("/withdraw", h.RequestWithdrawal)

	// Admin payout routing
	admin := router.Group("/admin/payouts", authMiddleware)
	admin.Get("/requests", h.ListWithdrawalRequestsAdmin)
	admin.Put("/requests/:id/process", h.ProcessWithdrawalAdmin)
}

func (h *Handler) GetWalletBalance(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	role := c.Locals("role").(string)

	summary, err := h.service.GetWalletBalance(c.Context(), userID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load wallet", nil)
	}

	return response.Success(c, fiber.StatusOK, "Wallet summary loaded", fiber.Map{
		"owner_id":               summary.OwnerID,
		"owner_type":             role,
		"available_balance":      summary.AvailableBalance,
		"pending_escrow_balance": summary.PendingEscrowBalance,
		"locked_balance":         summary.LockedBalance,
		"total_withdrawn":        summary.TotalWithdrawn,
		"currency":               summary.Currency,
	})
}

func (h *Handler) GetLedgerHistory(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	txs, err := h.service.GetLedgerHistory(c.Context(), userID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load transactions", nil)
	}
	return response.Success(c, fiber.StatusOK, "Wallet ledger loaded", fiber.Map{"transactions": txs, "count": len(txs)})
}

type WithdrawReq struct {
	Amount         float64 `json:"amount"`
	PaymentMethod  string  `json:"payment_method"`
	AccountDetails string  `json:"account_details"`
}

func (h *Handler) RequestWithdrawal(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	var req WithdrawReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}
	if req.Amount <= 0 || req.PaymentMethod == "" || req.AccountDetails == "" {
		return response.ValidationError(c, map[string]string{
			"withdrawal": "amount, payment_method, and account_details are required",
		})
	}

	reqID, err := h.service.RequestWithdrawal(c.Context(), userID, req.Amount, req.PaymentMethod, req.AccountDetails)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to create withdrawal request: "+err.Error(), nil)
	}

	return response.Created(c, "Payout withdrawal request submitted", fiber.Map{
		"withdrawal_id":    reqID,
		"user_id":          userID,
		"requested_amount": req.Amount,
		"payment_method":   req.PaymentMethod,
		"status":           "PENDING",
	})
}

func (h *Handler) ListWithdrawalRequestsAdmin(c *fiber.Ctx) error {
	reqs, err := h.service.ListWithdrawalRequestsAdmin(c.Context())
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to fetch payout requests", nil)
	}
	return response.Success(c, fiber.StatusOK, "Payout requests loaded", fiber.Map{"requests": reqs, "count": len(reqs)})
}

type ProcessWithdrawalReq struct {
	Status              string `json:"status"`
	TransactionProofRef string `json:"transaction_proof_ref"`
}

func (h *Handler) ProcessWithdrawalAdmin(c *fiber.Ctx) error {
	withdrawalID := c.Params("id")
	var req ProcessWithdrawalReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid payload: "+err.Error())
	}
	if req.Status == "" {
		req.Status = "APPROVED"
	}

	if err := h.service.ProcessWithdrawalAdmin(c.Context(), withdrawalID, req.Status, req.TransactionProofRef); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to process withdrawal", nil)
	}

	return response.Success(c, fiber.StatusOK, "Payout withdrawal processed", fiber.Map{
		"withdrawal_id":         withdrawalID,
		"status":                req.Status,
		"transaction_proof_ref": req.TransactionProofRef,
	})
}
