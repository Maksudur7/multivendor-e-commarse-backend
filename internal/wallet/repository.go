package wallet

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

type WalletSummary struct {
	OwnerID               string  `json:"owner_id"`
	AvailableBalance      float64 `json:"available_balance"`
	PendingEscrowBalance  float64 `json:"pending_escrow_balance"`
	LockedBalance         float64 `json:"locked_balance"`
	TotalWithdrawn        float64 `json:"total_withdrawn"`
	Currency              string  `json:"currency"`
}

func (r *Repository) GetWalletBalance(ctx context.Context, userID string) (*WalletSummary, error) {
	if r.db == nil {
		return &WalletSummary{OwnerID: userID, AvailableBalance: 0, PendingEscrowBalance: 0, TotalWithdrawn: 0, Currency: "BDT"}, nil
	}
	var available, pending, totalWithdrawn float64
	err := r.db.QueryRow(ctx, `
		SELECT available_balance, pending_escrow, total_withdrawn FROM wallets WHERE user_id::text = $1`, userID).
		Scan(&available, &pending, &totalWithdrawn)
	if err != nil {
		_, _ = r.db.Exec(ctx, "INSERT INTO wallets (user_id, available_balance, pending_escrow, total_withdrawn) VALUES ($1::uuid, 0, 0, 0) ON CONFLICT (user_id) DO NOTHING", userID)
		available, pending, totalWithdrawn = 0, 0, 0
	}
	return &WalletSummary{
		OwnerID:              userID,
		AvailableBalance:     available,
		PendingEscrowBalance: pending,
		LockedBalance:        0.00,
		TotalWithdrawn:       totalWithdrawn,
		Currency:             "BDT",
	}, nil
}

type TransactionItem struct {
	TransactionID string  `json:"transaction_id"`
	Amount        float64 `json:"amount"`
	Type          string  `json:"type"`
	Description   string  `json:"description"`
	CreatedAt     string  `json:"created_at"`
}

func (r *Repository) GetLedgerHistory(ctx context.Context, userID string) ([]TransactionItem, error) {
	if r.db == nil {
		return []TransactionItem{}, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT t.id::text, t.amount, t.type, COALESCE(t.description, ''), t.created_at
		FROM wallet_transactions t JOIN wallets w ON w.id = t.wallet_id WHERE w.user_id::text = $1 ORDER BY t.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []TransactionItem
	for rows.Next() {
		var item TransactionItem
		var dt time.Time
		if err := rows.Scan(&item.TransactionID, &item.Amount, &item.Type, &item.Description, &dt); err == nil {
			item.CreatedAt = dt.Format(time.RFC3339)
			list = append(list, item)
		}
	}
	if list == nil { list = []TransactionItem{} }
	return list, nil
}

func (r *Repository) RequestWithdrawal(ctx context.Context, userID string, amount float64, method, details string) (string, error) {
	if r.db == nil {
		return "", errors.New("database connection unavailable")
	}
	var reqID string
	err := r.db.QueryRow(ctx, `
		INSERT INTO payout_requests (user_id, amount, payment_method, account_details, status)
		VALUES ($1::uuid, $2, $3, $4, 'PENDING')
		RETURNING id::text`,
		userID, amount, method, details,
	).Scan(&reqID)
	if err != nil {
		return "", err
	}
	return reqID, nil
}

type AdminPayoutRequestItem struct {
	WithdrawalID   string  `json:"withdrawal_id"`
	UserID         string  `json:"user_id"`
	Amount         float64 `json:"amount"`
	PaymentMethod  string  `json:"payment_method"`
	AccountDetails string  `json:"account_details"`
	Status         string  `json:"status"`
	Email          string  `json:"email"`
	CreatedAt      string  `json:"created_at"`
}

func (r *Repository) ListWithdrawalRequestsAdmin(ctx context.Context) ([]AdminPayoutRequestItem, error) {
	if r.db == nil {
		return []AdminPayoutRequestItem{}, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT p.id::text, p.user_id::text, p.amount, p.payment_method, p.account_details, p.status, p.created_at, COALESCE(u.email,'')
		FROM payout_requests p LEFT JOIN users u ON u.id = p.user_id ORDER BY p.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []AdminPayoutRequestItem
	for rows.Next() {
		var item AdminPayoutRequestItem
		var dt time.Time
		if err := rows.Scan(&item.WithdrawalID, &item.UserID, &item.Amount, &item.PaymentMethod, &item.AccountDetails, &item.Status, &dt, &item.Email); err == nil {
			item.CreatedAt = dt.Format(time.RFC3339)
			list = append(list, item)
		}
	}
	if list == nil { list = []AdminPayoutRequestItem{} }
	return list, nil
}

func (r *Repository) ProcessWithdrawalAdmin(ctx context.Context, withdrawalID, status, proofRef string) error {
	if r.db == nil { return nil }
	_, err := r.db.Exec(ctx, "UPDATE payout_requests SET status = $1, transaction_proof_ref = $2 WHERE id::text = $3", status, proofRef, withdrawalID)
	return err
}
