package vendor

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

type PublicStore struct {
	VendorID           string  `json:"vendor_id"`
	StoreName          string  `json:"store_name"`
	StoreSlug          string  `json:"store_slug"`
	Description        string  `json:"description"`
	LogoURL            string  `json:"logo_url"`
	VerificationStatus string  `json:"verification_status"`
	Rating             float64 `json:"rating"`
}

func (r *Repository) GetStoreBySlug(ctx context.Context, slug string) (*PublicStore, error) {
	if r.db == nil {
		return &PublicStore{
			StoreSlug: slug, StoreName: "Official Store", VerificationStatus: "APPROVED", Rating: 4.8,
		}, nil
	}
	var s PublicStore
	s.StoreSlug = slug
	err := r.db.QueryRow(ctx, `
		SELECT id::text, store_name, COALESCE(description,''), COALESCE(logo_url,''), verification_status, store_rating
		FROM vendors WHERE store_slug = $1`, slug).Scan(&s.VendorID, &s.StoreName, &s.Description, &s.LogoURL, &s.VerificationStatus, &s.Rating)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

type VendorProfile struct {
	Registered         bool    `json:"registered"`
	VendorID           string  `json:"vendor_id,omitempty"`
	StoreName          string  `json:"store_name,omitempty"`
	StoreSlug          string  `json:"store_slug,omitempty"`
	VerificationStatus string  `json:"verification_status,omitempty"`
	CommissionRate     float64 `json:"commission_rate,omitempty"`
}

func (r *Repository) GetVendorByUserID(ctx context.Context, userID string) (*VendorProfile, error) {
	if r.db == nil {
		return &VendorProfile{Registered: true, VendorID: "mock-vendor", VerificationStatus: "APPROVED"}, nil
	}
	var vp VendorProfile
	err := r.db.QueryRow(ctx, `
		SELECT id::text, store_name, store_slug, verification_status, commission_rate
		FROM vendors WHERE user_id::text = $1`, userID).Scan(&vp.VendorID, &vp.StoreName, &vp.StoreSlug, &vp.VerificationStatus, &vp.CommissionRate)
	if err != nil {
		return &VendorProfile{Registered: false}, nil
	}
	vp.Registered = true
	return &vp, nil
}

type RegisterStoreParams struct {
	UserID              string
	StoreName           string
	StoreSlug           string
	Description         string
	BankName            string
	BankAccountNumber   string
	BkashMerchantNumber string
}

func (r *Repository) RegisterStore(ctx context.Context, p RegisterStoreParams) (string, error) {
	if r.db == nil {
		return "mock-vendor-id", nil
	}
	var vendorID string
	err := r.db.QueryRow(ctx, `
		INSERT INTO vendors (user_id, store_name, store_slug, description, bank_name, bank_account_number, bkash_merchant_number, verification_status)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, 'PENDING')
		ON CONFLICT (user_id) DO UPDATE SET store_name = EXCLUDED.store_name, store_slug = EXCLUDED.store_slug
		RETURNING id::text`,
		p.UserID, p.StoreName, p.StoreSlug, p.Description, p.BankName, p.BankAccountNumber, p.BkashMerchantNumber,
	).Scan(&vendorID)
	if err != nil {
		return "", err
	}

	_, _ = r.db.Exec(ctx, "UPDATE users SET role = 'VENDOR' WHERE id::text = $1", p.UserID)
	return vendorID, nil
}

func (r *Repository) UpdateStore(ctx context.Context, userID, name, desc string) error {
	if r.db == nil { return nil }
	_, err := r.db.Exec(ctx, `
		UPDATE vendors SET store_name = COALESCE(NULLIF($1,''), store_name), description = COALESCE(NULLIF($2,''), description)
		WHERE user_id::text = $3`, name, desc, userID)
	return err
}

type VendorAnalytics struct {
	Registered      bool    `json:"registered"`
	VendorID        string  `json:"vendor_id,omitempty"`
	WalletBalance   float64 `json:"wallet_balance"`
	PendingEscrow   float64 `json:"pending_escrow"`
	TotalOrders     int     `json:"total_orders"`
	PendingOrders   int     `json:"pending_orders"`
	CompletedOrders int     `json:"completed_orders"`
	TotalRevenue    float64 `json:"total_revenue"`
}

func (r *Repository) GetVendorAnalytics(ctx context.Context, userID string) (*VendorAnalytics, error) {
	if r.db == nil {
		return &VendorAnalytics{Registered: true, VendorID: "mock-vendor", WalletBalance: 0, TotalOrders: 0}, nil
	}
	var vendorID string
	err := r.db.QueryRow(ctx, "SELECT id::text FROM vendors WHERE user_id::text = $1", userID).Scan(&vendorID)
	if err != nil {
		return &VendorAnalytics{Registered: false}, nil
	}

	var va VendorAnalytics
	va.Registered = true
	va.VendorID = vendorID

	_ = r.db.QueryRow(ctx, "SELECT available_balance, COALESCE(pending_escrow,0) FROM wallets WHERE user_id::text = $1", userID).Scan(&va.WalletBalance, &va.PendingEscrow)
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*), COALESCE(SUM(grand_total),0) FROM master_orders WHERE vendor_id::text = $1`, vendorID).Scan(&va.TotalOrders, &va.TotalRevenue)
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM master_orders WHERE vendor_id::text = $1 AND status = 'PENDING'`, vendorID).Scan(&va.PendingOrders)
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM master_orders WHERE vendor_id::text = $1 AND status = 'DELIVERED'`, vendorID).Scan(&va.CompletedOrders)

	return &va, nil
}

type AdminVendorItem struct {
	VendorID           string  `json:"vendor_id"`
	StoreName          string  `json:"store_name"`
	StoreSlug          string  `json:"store_slug"`
	VerificationStatus string  `json:"status"`
	CommissionRate     float64 `json:"commission_rate"`
	Email              string  `json:"email"`
	CreatedAt          string  `json:"created_at"`
}

func (r *Repository) ListVendorsAdmin(ctx context.Context) ([]AdminVendorItem, error) {
	if r.db == nil {
		return []AdminVendorItem{}, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT v.id::text, v.store_name, v.store_slug, v.verification_status, v.commission_rate, v.created_at, COALESCE(u.email,'')
		FROM vendors v LEFT JOIN users u ON u.id = v.user_id ORDER BY v.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []AdminVendorItem
	for rows.Next() {
		var item AdminVendorItem
		var dt time.Time
		if err := rows.Scan(&item.VendorID, &item.StoreName, &item.StoreSlug, &item.VerificationStatus, &item.CommissionRate, &dt, &item.Email); err == nil {
			item.CreatedAt = dt.Format(time.RFC3339)
			list = append(list, item)
		}
	}
	if list == nil { list = []AdminVendorItem{} }
	return list, nil
}

func (r *Repository) VerifyVendor(ctx context.Context, vendorID, status string, rate float64) error {
	if r.db == nil { return nil }
	_, err := r.db.Exec(ctx, "UPDATE vendors SET verification_status = $1, commission_rate = $2 WHERE id::text = $3", status, rate, vendorID)
	return err
}
