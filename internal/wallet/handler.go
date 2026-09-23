package wallet

import (
		"time"

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

	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Wallet summary loaded", fiber.Map{
			"owner_id": userID, "available_balance": 0.0, "currency": "BDT",
		})
	}

	ctx := c.Context()
	var available, pending, totalWithdrawn float64
	err := h.db.QueryRow(ctx, `
		SELECT available_balance, pending_escrow, total_withdrawn FROM wallets WHERE user_id::text = $1`, userID).
		Scan(&available, &pending, &totalWithdrawn)
	if err != nil {
		// Auto create wallet row if missing
		h.db.Exec(ctx, "INSERT INTO wallets (user_id, available_balance, pending_escrow, total_withdrawn) VALUES ($1::uuid, 0, 0, 0) ON CONFLICT (user_id) DO NOTHING", userID)
	}

	return response.Success(c, fiber.StatusOK, "Wallet summary loaded from NeonDB", fiber.Map{
		"owner_id":               userID,
		"owner_type":             role,
		"available_balance":      available,
		"pending_escrow_balance": pending,
		"locked_balance":         0.00,
		"total_withdrawn":        totalWithdrawn,
		"currency":               "BDT",
	})
}

func (h *Handler) GetLedgerHistory(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Wallet ledger history", fiber.Map{"transactions": []fiber.Map{}})
	}
	ctx := c.Context()
	rows, err := h.db.Query(ctx, `
		SELECT t.id::text, t.amount, t.type, COALESCE(t.description, ''), t.created_at
		FROM wallet_transactions t JOIN wallets w ON w.id = t.wallet_id WHERE w.user_id::text = $1 ORDER BY t.created_at DESC`, userID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load transactions", nil)
	}
	defer rows.Close()

	txs := []fiber.Map{}
	for rows.Next() {
		var id, tType, desc string; var amt float64; var dt time.Time
		rows.Scan(&id, &amt, &tType, &desc, &dt)
		txs = append(txs, fiber.Map{"transaction_id": id, "amount": amt, "type": tType, "description": desc, "created_at": dt.Format(time.RFC3339)})
	}
	return response.Success(c, fiber.StatusOK, "Wallet ledger from NeonDB", fiber.Map{"transactions": txs, "count": len(txs)})
}

type WithdrawReq struct {
	Amount         float64 `json:"amount"`
	PaymentMethod  string  `json:"payment_method"`  // BKASH_MERCHANT, NAGAD, BANK_TRANSFER
	AccountDetails string  `json:"account_details"` // e.g. "Dutch-Bangla Bank A/C 123456789"
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

	if h.db == nil {
		return response.Created(c, "Withdrawal requested", fiber.Map{"requested_amount": req.Amount, "status": "PENDING"})
	}

	ctx := c.Context()
	var reqID string
	err := h.db.QueryRow(ctx, `
		INSERT INTO payout_requests (user_id, amount, payment_method, account_details, status)
		VALUES ($1::uuid, $2, $3, $4, 'PENDING')
		RETURNING id::text`,
		userID, req.Amount, req.PaymentMethod, req.AccountDetails,
	).Scan(&reqID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to create withdrawal request: "+err.Error(), nil)
	}

	return response.Created(c, "Payout withdrawal request submitted in NeonDB", fiber.Map{
		"withdrawal_id":    reqID,
		"user_id":          userID,
		"requested_amount": req.Amount,
		"payment_method":   req.PaymentMethod,
		"status":           "PENDING",
	})
}

func (h *Handler) ListWithdrawalRequestsAdmin(c *fiber.Ctx) error {
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Withdrawal requests for admin review", fiber.Map{"requests": []fiber.Map{}})
	}
	ctx := c.Context()
	rows, err := h.db.Query(ctx, `
		SELECT p.id::text, p.user_id::text, p.amount, p.payment_method, p.account_details, p.status, p.created_at, COALESCE(u.email,'')
		FROM payout_requests p LEFT JOIN users u ON u.id = p.user_id ORDER BY p.created_at DESC`)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to fetch payout requests", nil)
	}
	defer rows.Close()

	reqs := []fiber.Map{}
	for rows.Next() {
		var id, uID, method, details, status, email string
		var amt float64
		var dt time.Time
		rows.Scan(&id, &uID, &amt, &method, &details, &status, &dt, &email)
		reqs = append(reqs, fiber.Map{
			"withdrawal_id": id, "user_id": uID, "amount": amt, "payment_method": method, "account_details": details, "status": status, "email": email, "created_at": dt.Format(time.RFC3339),
		})
	}
	return response.Success(c, fiber.StatusOK, "Payout requests from NeonDB", fiber.Map{"requests": reqs, "count": len(reqs)})
}

type ProcessWithdrawalReq struct {
	Status              string `json:"status"` // APPROVED, REJECTED
	TransactionProofRef string `json:"transaction_proof_ref"`
}

func (h *Handler) ProcessWithdrawalAdmin(c *fiber.Ctx) error {
	withdrawalID := c.Params("id")
	var req ProcessWithdrawalReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid payload: "+err.Error())
	}
	if req.Status == "" { req.Status = "APPROVED" }

	if h.db != nil {
		ctx := c.Context()
		h.db.Exec(ctx, "UPDATE payout_requests SET status = $1, transaction_proof_ref = $2 WHERE id::text = $3", req.Status, req.TransactionProofRef, withdrawalID)
	}
	return response.Success(c, fiber.StatusOK, "Payout withdrawal processed in NeonDB", fiber.Map{
		"withdrawal_id":         withdrawalID,
		"status":                req.Status,
		"transaction_proof_ref": req.TransactionProofRef,
	})
}
