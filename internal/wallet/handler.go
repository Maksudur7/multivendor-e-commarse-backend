package wallet

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

	return response.Success(c, fiber.StatusOK, "Wallet summary loaded", fiber.Map{
		"owner_id":               userID,
		"owner_type":             role,
		"available_balance":      24500.00,
		"pending_escrow_balance": 18200.00,
		"locked_balance":         0.00,
		"total_withdrawn":        54000.00,
		"currency":               "BDT",
	})
}

func (h *Handler) GetLedgerHistory(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Wallet ledger history", fiber.Map{
		"transactions": []fiber.Map{},
	})
}

type WithdrawReq struct {
	Amount         float64 `json:"amount"`
	PaymentMethod  string  `json:"payment_method"`  // BKASH_MERCHANT, NAGAD, BANK_TRANSFER
	AccountDetails string  `json:"account_details"` // e.g. "Dutch-Bangla Bank A/C 123456789"
}

func (h *Handler) RequestWithdrawal(c *fiber.Ctx) error {
	var req WithdrawReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}
	if req.Amount <= 0 || req.PaymentMethod == "" || req.AccountDetails == "" {
		return response.ValidationError(c, map[string]string{
			"withdrawal": "amount, payment_method, and account_details are required",
		})
	}

	userID := c.Locals("user_id").(string)

	return response.Created(c, "Payout withdrawal request submitted. Will be processed within 24 hours.", fiber.Map{
		"withdrawal_id":   uuid.New().String(),
		"user_id":         userID,
		"requested_amount": req.Amount,
		"payment_method":  req.PaymentMethod,
		"status":          "PENDING",
	})
}

func (h *Handler) ListWithdrawalRequestsAdmin(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Withdrawal requests for admin review", fiber.Map{
		"requests": []fiber.Map{},
	})
}

type ProcessWithdrawalReq struct {
	Status             string `json:"status"` // APPROVED, REJECTED
	TransactionProofRef string `json:"transaction_proof_ref"`
}

func (h *Handler) ProcessWithdrawalAdmin(c *fiber.Ctx) error {
	withdrawalID := c.Params("id")
	var req ProcessWithdrawalReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid payload: "+err.Error())
	}
	return response.Success(c, fiber.StatusOK, "Payout withdrawal processed", fiber.Map{
		"withdrawal_id":         withdrawalID,
		"status":                req.Status,
		"transaction_proof_ref": req.TransactionProofRef,
	})
}
