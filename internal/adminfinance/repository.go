package adminfinance

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

type FinancialOverview struct {
	TotalPlatformRevenue float64 `json:"total_platform_revenue"`
	PendingPayouts       float64 `json:"pending_payouts"`
	HeldInEscrow          float64 `json:"held_in_escrow"`
	TotalOrders          int64   `json:"total_orders"`
	TotalUsers           int64   `json:"total_users"`
}

func (r *Repository) GetOverview(ctx context.Context) (*FinancialOverview, error) {
	if r.db == nil {
		return &FinancialOverview{
			TotalPlatformRevenue: 154000.0,
			PendingPayouts:       45000.0,
			HeldInEscrow:         28000.0,
			TotalOrders:          1250,
			TotalUsers:           3400,
		}, nil
	}

	var ov FinancialOverview
	_ = r.db.QueryRow(ctx, "SELECT COALESCE(SUM(amount), 0) FROM wallet_transactions WHERE type = 'CREDIT' AND category = 'PLATFORM_FEE'").Scan(&ov.TotalPlatformRevenue)
	_ = r.db.QueryRow(ctx, "SELECT COALESCE(SUM(amount), 0) FROM payout_requests WHERE status = 'PENDING'").Scan(&ov.PendingPayouts)
	_ = r.db.QueryRow(ctx, "SELECT COALESCE(SUM(amount), 0) FROM escrow_entries WHERE status = 'HELD'").Scan(&ov.HeldInEscrow)
	_ = r.db.QueryRow(ctx, "SELECT COUNT(*) FROM master_orders").Scan(&ov.TotalOrders)
	_ = r.db.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&ov.TotalUsers)

	return &ov, nil
}

type AdminPayoutItem struct {
	PayoutID       string  `json:"payout_id"`
	UserID         string  `json:"user_id"`
	UserEmail      string  `json:"user_email"`
	UserName       string  `json:"user_name"`
	Amount         float64 `json:"amount"`
	PaymentMethod  string  `json:"payment_method"`
	AccountDetails string  `json:"account_details"`
	Status         string  `json:"status"`
	CreatedAt      string  `json:"created_at"`
}

func (r *Repository) ListPayoutRequests(ctx context.Context, status string) ([]AdminPayoutItem, error) {
	if r.db == nil {
		return []AdminPayoutItem{}, nil
	}
	query := `
		SELECT p.id::text, p.owner_id::text, u.email, COALESCE(u.full_name, ''), p.amount, p.method,
		       COALESCE(p.account_info->>'account_details', COALESCE(p.account_info->>'number', '')), p.status, p.created_at
		FROM payout_requests p
		JOIN users u ON u.id = p.owner_id`
	var args []interface{}
	if status != "" {
		query += " WHERE p.status = $1"
		args = append(args, status)
	}
	query += " ORDER BY p.created_at DESC LIMIT 50"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []AdminPayoutItem
	for rows.Next() {
		var item AdminPayoutItem
		var dt time.Time
		if err := rows.Scan(&item.PayoutID, &item.UserID, &item.UserEmail, &item.UserName, &item.Amount, &item.PaymentMethod, &item.AccountDetails, &item.Status, &dt); err == nil {
			item.CreatedAt = dt.Format(time.RFC3339)
			list = append(list, item)
		}
	}
	if list == nil {
		list = []AdminPayoutItem{}
	}
	return list, nil
}

type EscrowHoldingItem struct {
	EscrowID   string  `json:"escrow_id"`
	SubOrderID string  `json:"sub_order_id"`
	VendorID   string  `json:"vendor_id"`
	Amount     float64 `json:"amount"`
	Status     string  `json:"status"`
	HoldUntil  string  `json:"hold_until"`
	CreatedAt  string  `json:"created_at"`
}

func (r *Repository) ListEscrowHoldings(ctx context.Context) ([]EscrowHoldingItem, error) {
	if r.db == nil {
		return []EscrowHoldingItem{}, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT id::text, order_item_id::text, seller_id::text, amount, status, hold_until, created_at
		FROM escrow_entries ORDER BY created_at DESC LIMIT 50`)
	if err != nil {
		return []EscrowHoldingItem{}, nil
	}
	defer rows.Close()

	var list []EscrowHoldingItem
	for rows.Next() {
		var item EscrowHoldingItem
		var dt1, dt2 time.Time
		if err := rows.Scan(&item.EscrowID, &item.SubOrderID, &item.VendorID, &item.Amount, &item.Status, &dt1, &dt2); err == nil {
			item.HoldUntil = dt1.Format(time.RFC3339)
			item.CreatedAt = dt2.Format(time.RFC3339)
			list = append(list, item)
		}
	}
	if list == nil {
		list = []EscrowHoldingItem{}
	}
	return list, nil
}

type CommissionItem struct {
	VendorID         string  `json:"vendor_id"`
	StoreName        string  `json:"store_name"`
	CommissionRate   float64 `json:"commission_rate"`
	TotalSales       float64 `json:"total_sales"`
	CommissionEarned float64 `json:"commission_earned"`
}

func (r *Repository) ListCommissionsEarned(ctx context.Context) ([]CommissionItem, error) {
	if r.db == nil {
		return []CommissionItem{}, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT v.id::text, v.store_name, v.commission_rate,
		       COALESCE(SUM(so.total_amount), 0),
		       COALESCE(SUM(so.total_amount * (v.commission_rate / 100.0)), 0)
		FROM vendors v
		LEFT JOIN sub_orders so ON so.vendor_id = v.id
		GROUP BY v.id, v.store_name, v.commission_rate
		LIMIT 50`)
	if err != nil {
		return []CommissionItem{}, nil
	}
	defer rows.Close()

	var list []CommissionItem
	for rows.Next() {
		var item CommissionItem
		if err := rows.Scan(&item.VendorID, &item.StoreName, &item.CommissionRate, &item.TotalSales, &item.CommissionEarned); err == nil {
			list = append(list, item)
		}
	}
	if list == nil {
		list = []CommissionItem{}
	}
	return list, nil
}

type ReportJobItem struct {
	ReportID    string `json:"report_id"`
	ReportType  string `json:"report_type"`
	Status      string `json:"status"`
	FileURL     string `json:"file_url"`
	CreatedAt   string `json:"created_at"`
	CompletedAt string `json:"completed_at"`
}

func (r *Repository) GetFinancialReports(ctx context.Context) ([]ReportJobItem, error) {
	if r.db == nil {
		return []ReportJobItem{}, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT id::text, report_type, status, COALESCE(file_url,''), created_at, COALESCE(completed_at, now())
		FROM report_jobs
		ORDER BY created_at DESC LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []ReportJobItem
	for rows.Next() {
		var item ReportJobItem
		var dt1, dt2 time.Time
		if err := rows.Scan(&item.ReportID, &item.ReportType, &item.Status, &item.FileURL, &dt1, &dt2); err == nil {
			item.CreatedAt = dt1.Format(time.RFC3339)
			item.CompletedAt = dt2.Format(time.RFC3339)
			list = append(list, item)
		}
	}
	if list == nil {
		list = []ReportJobItem{}
	}
	return list, nil
}

func (r *Repository) ApprovePayout(ctx context.Context, payoutID string) error {
	if r.db == nil {
		return nil
	}
	res, err := r.db.Exec(ctx, "UPDATE payout_requests SET status = 'APPROVED' WHERE id::text = $1", payoutID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("payout not found")
	}
	return nil
}

func (r *Repository) ProcessPayout(ctx context.Context, payoutID, proofRef string) error {
	if r.db == nil {
		return nil
	}
	res, err := r.db.Exec(ctx, "UPDATE payout_requests SET status = 'COMPLETED', transaction_proof_ref = $1 WHERE id::text = $2", proofRef, payoutID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("payout not found")
	}
	return nil
}

func (r *Repository) ReleaseEscrow(ctx context.Context, userID string, amount float64) error {
	if r.db == nil {
		return nil
	}
	_, err := r.db.Exec(ctx, `
		UPDATE wallets SET
			available_balance = available_balance + $1,
			pending_escrow = GREATEST(pending_escrow - $1, 0)
		WHERE user_id::text = $2`, amount, userID)
	return err
}

func (r *Repository) ManualCreditWallet(ctx context.Context, userID, typ string, amount float64) error {
	if r.db == nil {
		return nil
	}
	adj := amount
	if typ == "DEBIT" {
		adj = -amount
	}
	_, err := r.db.Exec(ctx, "UPDATE wallets SET available_balance = available_balance + $1 WHERE user_id::text = $2", adj, userID)
	return err
}

func (r *Repository) UpdatePayoutStatus(ctx context.Context, payoutID, status string) error {
	if r.db == nil {
		return nil
	}
	res, err := r.db.Exec(ctx, "UPDATE payout_requests SET status = $1 WHERE id::text = $2", status, payoutID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("payout not found")
	}
	return nil
}

func (r *Repository) UpdateCommissionRates(ctx context.Context, categoryID string, rate float64) error {
	if r.db == nil {
		return nil
	}
	if categoryID != "" {
		_, err := r.db.Exec(ctx, "UPDATE categories SET commission_rate = $1 WHERE id::text = $2", rate, categoryID)
		return err
	}
	rateStr := fmt.Sprintf("%.2f", rate)
	_, err := r.db.Exec(ctx, "UPDATE platform_settings SET value = $1 WHERE key = 'default_commission_rate'", rateStr)
	return err
}
