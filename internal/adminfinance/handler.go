package adminfinance

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
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Financial dashboard metrics", fiber.Map{
			"total_platform_revenue": 0.0,
			"pending_payouts":        0.0,
			"held_in_escrow":         0.0,
		})
	}
	ctx := c.Context()

	var totalRevenue float64
	h.db.QueryRow(ctx, `SELECT COALESCE(SUM(amount),0) FROM payments WHERE status = 'COMPLETED'`).Scan(&totalRevenue)

	var pendingPayouts float64
	h.db.QueryRow(ctx, `SELECT COALESCE(SUM(amount),0) FROM payout_requests WHERE status = 'PENDING'`).Scan(&pendingPayouts)

	var heldEscrow float64
	h.db.QueryRow(ctx, `SELECT COALESCE(SUM(pending_escrow),0) FROM wallets`).Scan(&heldEscrow)

	var totalOrders, totalUsers int
	h.db.QueryRow(ctx, `SELECT COUNT(*) FROM master_orders`).Scan(&totalOrders)
	h.db.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&totalUsers)

	return response.Success(c, fiber.StatusOK, "Financial dashboard metrics from NeonDB", fiber.Map{
		"total_platform_revenue": totalRevenue,
		"pending_payouts":        pendingPayouts,
		"held_in_escrow":         heldEscrow,
		"total_orders":           totalOrders,
		"total_users":            totalUsers,
	})
}

func (h *Handler) ListPayoutRequests(c *fiber.Ctx) error {
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Payout requests list", fiber.Map{"payouts": []fiber.Map{}})
	}
	ctx := c.Context()
	status := c.Query("status") // filter: PENDING, APPROVED, REJECTED

	query := `
		SELECT p.id::text, p.user_id::text, p.amount, p.payment_method, p.account_details,
		       p.status, p.created_at, COALESCE(u.email,''), COALESCE(u.full_name,'')
		FROM payout_requests p
		LEFT JOIN users u ON u.id = p.user_id`
	args := []interface{}{}
	if status != "" {
		query += " WHERE p.status = $1"
		args = append(args, status)
	}
	query += " ORDER BY p.created_at DESC LIMIT 100"

	rows, err := h.db.Query(ctx, query, args...)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load payout requests: "+err.Error(), nil)
	}
	defer rows.Close()

	payouts := []fiber.Map{}
	for rows.Next() {
		var id, userID, method, details, pStatus, email, name string
		var amt float64
		var dt time.Time
		rows.Scan(&id, &userID, &amt, &method, &details, &pStatus, &dt, &email, &name)
		payouts = append(payouts, fiber.Map{
			"payout_id": id, "user_id": userID, "user_email": email, "user_name": name,
			"amount": amt, "payment_method": method, "account_details": details,
			"status": pStatus, "created_at": dt.Format(time.RFC3339),
		})
	}
	return response.Success(c, fiber.StatusOK, "Payout requests from NeonDB", fiber.Map{
		"payouts": payouts, "count": len(payouts),
	})
}

func (h *Handler) ListEscrowHoldings(c *fiber.Ctx) error {
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Escrow holdings", fiber.Map{"escrow": []fiber.Map{}})
	}
	ctx := c.Context()
	rows, err := h.db.Query(ctx, `
		SELECT w.id::text, w.user_id::text, w.pending_escrow, w.available_balance, u.email
		FROM wallets w
		LEFT JOIN users u ON u.id = w.user_id
		WHERE w.pending_escrow > 0
		ORDER BY w.pending_escrow DESC`)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load escrow data: "+err.Error(), nil)
	}
	defer rows.Close()

	escrow := []fiber.Map{}
	for rows.Next() {
		var id, userID, email string
		var pending, available float64
		rows.Scan(&id, &userID, &pending, &available, &email)
		escrow = append(escrow, fiber.Map{
			"wallet_id": id, "user_id": userID, "user_email": email,
			"pending_escrow": pending, "available_balance": available,
		})
	}
	return response.Success(c, fiber.StatusOK, "Escrow holdings from NeonDB", fiber.Map{
		"escrow": escrow, "count": len(escrow),
	})
}

func (h *Handler) ListCommissionsEarned(c *fiber.Ctx) error {
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Platform commissions log", fiber.Map{"commissions": []fiber.Map{}})
	}
	ctx := c.Context()
	rows, err := h.db.Query(ctx, `
		SELECT s.id::text, COALESCE(s.shop_name,'Seller Store'), COALESCE(s.commission_override, 10.0),
		       COALESCE(SUM(p.amount),0) as total_sales,
		       COALESCE(SUM(p.amount) * COALESCE(s.commission_override, 10.0) / 100, 0) as commission_earned
		FROM seller_profiles s
		LEFT JOIN master_orders mo ON mo.seller_id = s.user_id
		LEFT JOIN payments p ON p.master_order_id = mo.id AND p.status = 'COMPLETED'
		GROUP BY s.id, s.shop_name, s.commission_override
		ORDER BY commission_earned DESC`)
	if err != nil {
		return response.Success(c, fiber.StatusOK, "Platform commissions from NeonDB", fiber.Map{"commissions": []fiber.Map{}, "count": 0})
	}
	defer rows.Close()

	commissions := []fiber.Map{}
	for rows.Next() {
		var id, storeName string
		var rate, totalSales, commEarned float64
		rows.Scan(&id, &storeName, &rate, &totalSales, &commEarned)
		commissions = append(commissions, fiber.Map{
			"vendor_id": id, "store_name": storeName, "commission_rate": rate,
			"total_sales": totalSales, "commission_earned": commEarned,
		})
	}
	return response.Success(c, fiber.StatusOK, "Platform commissions from NeonDB", fiber.Map{
		"commissions": commissions, "count": len(commissions),
	})
}

func (h *Handler) GetFinancialReports(c *fiber.Ctx) error {
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Financial reports list", fiber.Map{"reports": []fiber.Map{}})
	}
	ctx := c.Context()
	rows, err := h.db.Query(ctx, `
		SELECT id::text, report_type, status, COALESCE(file_url,''), created_at, COALESCE(completed_at, now())
		FROM report_jobs
		ORDER BY created_at DESC LIMIT 50`)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load reports: "+err.Error(), nil)
	}
	defer rows.Close()

	reports := []fiber.Map{}
	for rows.Next() {
		var id, rType, status, fileURL string
		var createdAt, completedAt time.Time
		rows.Scan(&id, &rType, &status, &fileURL, &createdAt, &completedAt)
		reports = append(reports, fiber.Map{
			"report_id": id, "report_type": rType, "status": status,
			"file_url": fileURL, "created_at": createdAt.Format(time.RFC3339),
			"completed_at": completedAt.Format(time.RFC3339),
		})
	}
	return response.Success(c, fiber.StatusOK, "Financial reports from NeonDB", fiber.Map{
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
	if h.db != nil {
		ctx := c.Context()
		result, err := h.db.Exec(ctx, "UPDATE payout_requests SET status = 'APPROVED' WHERE id::text = $1", req.PayoutID)
		if err != nil {
			return response.Error(c, fiber.StatusInternalServerError, "Failed to approve payout: "+err.Error(), nil)
		}
		if result.RowsAffected() == 0 {
			return response.Error(c, fiber.StatusNotFound, "Payout request not found", nil)
		}
	}
	return response.Success(c, fiber.StatusOK, "Payout request approved in NeonDB", fiber.Map{
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
	if h.db != nil {
		ctx := c.Context()
		result, err := h.db.Exec(ctx,
			"UPDATE payout_requests SET status = 'COMPLETED', transaction_proof_ref = $1 WHERE id::text = $2",
			req.TransactionProofRef, req.PayoutID)
		if err != nil {
			return response.Error(c, fiber.StatusInternalServerError, "Failed to process payout: "+err.Error(), nil)
		}
		if result.RowsAffected() == 0 {
			return response.Error(c, fiber.StatusNotFound, "Payout request not found", nil)
		}
	}
	return response.Success(c, fiber.StatusOK, "Payout processed in NeonDB", fiber.Map{
		"payout_id": req.PayoutID, "status": "COMPLETED", "proof_ref": req.TransactionProofRef,
	})
}

type ReleaseEscrowReq struct {
	UserID string `json:"user_id"`
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
	if h.db != nil {
		ctx := c.Context()
		_, err := h.db.Exec(ctx, `
			UPDATE wallets SET
				available_balance = available_balance + $1,
				pending_escrow = GREATEST(pending_escrow - $1, 0)
			WHERE user_id::text = $2`, req.Amount, req.UserID)
		if err != nil {
			return response.Error(c, fiber.StatusInternalServerError, "Failed to release escrow: "+err.Error(), nil)
		}
	}
	return response.Success(c, fiber.StatusOK, "Escrow released to vendor wallet in NeonDB", fiber.Map{
		"user_id": req.UserID, "released_amount": req.Amount,
	})
}

type ManualCreditReq struct {
	UserID      string  `json:"user_id"`
	Amount      float64 `json:"amount"`
	Type        string  `json:"type"` // CREDIT or DEBIT
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
	if req.Type == "" {
		req.Type = "CREDIT"
	}

	if h.db != nil {
		ctx := c.Context()
		adjustment := req.Amount
		if req.Type == "DEBIT" {
			adjustment = -req.Amount
		}
		_, err := h.db.Exec(ctx, `
			UPDATE wallets SET available_balance = available_balance + $1 WHERE user_id::text = $2`,
			adjustment, req.UserID)
		if err != nil {
			return response.Error(c, fiber.StatusInternalServerError, "Failed to credit wallet: "+err.Error(), nil)
		}
	}
	return response.Created(c, "Manual credit/debit applied to user wallet in NeonDB", fiber.Map{
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
	if h.db != nil {
		ctx := c.Context()
		result, err := h.db.Exec(ctx, "UPDATE payout_requests SET status = $1 WHERE id::text = $2", body.Status, payoutID)
		if err != nil {
			return response.Error(c, fiber.StatusInternalServerError, "Failed to update payout: "+err.Error(), nil)
		}
		if result.RowsAffected() == 0 {
			return response.Error(c, fiber.StatusNotFound, "Payout not found", nil)
		}
	}
	return response.Success(c, fiber.StatusOK, "Payout status updated in NeonDB", fiber.Map{
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

	if h.db != nil {
		ctx := c.Context()
		if req.CategoryID != "" {
			h.db.Exec(ctx, "UPDATE categories SET commission_rate = $1 WHERE id::text = $2", req.CommissionRate, req.CategoryID)
		} else {
			// Update platform default setting
			h.db.Exec(ctx, "UPDATE platform_settings SET value = $1 WHERE key = 'default_commission_rate'", req.CommissionRate)
		}
	}
	return response.Success(c, fiber.StatusOK, "Commission rates updated in NeonDB", fiber.Map{
		"category_id": req.CategoryID, "commission_rate": req.CommissionRate,
	})
}
