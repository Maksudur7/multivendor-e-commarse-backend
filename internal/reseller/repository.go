package reseller

import (
	"context"

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
