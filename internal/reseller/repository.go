package reseller

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

type PublicResellerStore struct {
	ResellerID  string  `json:"reseller_id"`
	StoreName   string  `json:"store_name"`
	StoreSlug   string  `json:"store_slug"`
	TotalProfit float64 `json:"total_profit"`
}

func (r *Repository) GetPublicStore(ctx context.Context, slug string) (*PublicResellerStore, error) {
	if r.db == nil {
		return &PublicResellerStore{StoreSlug: slug, StoreName: "Reseller Shop", TotalProfit: 0.0}, nil
	}
	var s PublicResellerStore
	s.StoreSlug = slug
	err := r.db.QueryRow(ctx, "SELECT id::text, COALESCE(business_name, 'Reseller Shop'), total_earned FROM resellers WHERE id::text = $1 OR user_id::text = $1", slug).
		Scan(&s.ResellerID, &s.StoreName, &s.TotalProfit)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

type ResellerProfile struct {
	Registered       bool    `json:"registered"`
	ResellerID       string  `json:"reseller_id,omitempty"`
	StoreName        string  `json:"store_name,omitempty"`
	Status           string  `json:"status,omitempty"`
	TotalProfitEarned float64 `json:"total_profit_earned,omitempty"`
}

func (r *Repository) GetResellerByUserID(ctx context.Context, userID string) (*ResellerProfile, error) {
	if r.db == nil {
		return &ResellerProfile{Registered: true, ResellerID: "mock-reseller", TotalProfitEarned: 4580.0}, nil
	}
	var rp ResellerProfile
	err := r.db.QueryRow(ctx, "SELECT id::text, COALESCE(business_name, 'My Reseller Store'), status, total_earned FROM resellers WHERE user_id::text = $1", userID).
		Scan(&rp.ResellerID, &rp.StoreName, &rp.Status, &rp.TotalProfitEarned)
	if err != nil {
		return &ResellerProfile{Registered: false}, nil
	}
	rp.Registered = true
	return &rp, nil
}

// UserIDPlaceholder workaround tag helper
type ResellerProfileInternal struct {
	UserIDPlaceholder string
}

func (r *Repository) CreateStore(ctx context.Context, userID, name string) (string, error) {
	if r.db == nil {
		return "mock-store-id", nil
	}
	var storeID string
	err := r.db.QueryRow(ctx, `
		INSERT INTO resellers (user_id, status, business_name)
		VALUES ($1::uuid, 'ACTIVE', $2)
		ON CONFLICT (user_id) DO UPDATE SET business_name = EXCLUDED.business_name
		RETURNING id::text`,
		userID, name,
	).Scan(&storeID)
	if err != nil {
		return "", err
	}
	return storeID, nil
}

type ResellerAdminListItem struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	BusinessName string    `json:"business_name"`
	Status       string    `json:"status"`
	Tier         string    `json:"tier"`
	TotalOrders  int       `json:"total_orders"`
	TotalEarned  float64   `json:"total_earned"`
	CreatedAt    time.Time `json:"created_at"`
}

func (r *Repository) ListResellersAdmin(ctx context.Context, status string) ([]ResellerAdminListItem, error) {
	if r.db == nil {
		return []ResellerAdminListItem{}, nil
	}
	whereClause := ""
	args := []interface{}{}
	if status != "" {
		whereClause = " WHERE status = $1"
		args = append(args, status)
	}

	rows, err := r.db.Query(ctx, "SELECT id::text, user_id::text, COALESCE(business_name, ''), status, reseller_tier, total_orders, total_earned, created_at FROM resellers"+whereClause+" ORDER BY created_at DESC", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []ResellerAdminListItem{}
	for rows.Next() {
		var item ResellerAdminListItem
		if err := rows.Scan(&item.ID, &item.UserID, &item.BusinessName, &item.Status, &item.Tier, &item.TotalOrders, &item.TotalEarned, &item.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, nil
}

func (r *Repository) VerifyReseller(ctx context.Context, resellerID, status string) (int64, error) {
	if r.db == nil {
		return 0, nil
	}
	var userID string
	err := r.db.QueryRow(ctx, "UPDATE resellers SET status = $1, updated_at = now() WHERE id::text = $2 RETURNING user_id::text", status, resellerID).Scan(&userID)
	if err != nil {
		return 0, err
	}
	if status == "ACTIVE" && userID != "" {
		_, _ = r.db.Exec(ctx, "UPDATE users SET role = 'RESELLER', updated_at = now() WHERE id::text = $1", userID)
	}
	return 1, nil
}

