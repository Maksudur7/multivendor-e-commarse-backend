package promotion

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

type FlashSaleItem struct {
	SaleID           string  `json:"sale_id"`
	Title            string  `json:"title"`
	DiscountPct      float64 `json:"discount_pct"`
	StartsAt         string  `json:"starts_at"`
	EndsAt           string  `json:"ends_at"`
	IsActive         bool    `json:"is_active"`
	TimeRemainingSec int     `json:"time_remaining_sec"`
}

func (r *Repository) GetActiveFlashSales(ctx context.Context) ([]FlashSaleItem, error) {
	if r.db == nil {
		return []FlashSaleItem{}, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT id::text, title, discount_pct, starts_at, ends_at, is_active, created_at
		FROM flash_sales
		WHERE is_active = true AND ends_at > now()
		ORDER BY ends_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sales []FlashSaleItem
	for rows.Next() {
		var item FlashSaleItem
		var startsAt, endsAt, createdAt time.Time
		if err := rows.Scan(&item.SaleID, &item.Title, &item.DiscountPct, &startsAt, &endsAt, &item.IsActive, &createdAt); err == nil {
			item.StartsAt = startsAt.Format(time.RFC3339)
			item.EndsAt = endsAt.Format(time.RFC3339)
			item.TimeRemainingSec = int(time.Until(endsAt).Seconds())
			sales = append(sales, item)
		}
	}
	if sales == nil { sales = []FlashSaleItem{} }
	return sales, nil
}

type CouponData struct {
	Code           string
	DiscountType   string
	DiscountValue  float64
	MinOrderAmount float64
	IsActive       bool
}

func (r *Repository) GetCouponByCode(ctx context.Context, code string) (*CouponData, error) {
	if r.db == nil {
		return nil, nil
	}
	var c CouponData
	err := r.db.QueryRow(ctx, `
		SELECT code, discount_type, discount_value, min_order_amount, is_active
		FROM coupons WHERE UPPER(code) = UPPER($1) AND is_active = true`, code).
		Scan(&c.Code, &c.DiscountType, &c.DiscountValue, &c.MinOrderAmount, &c.IsActive)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

type AdminCouponItem struct {
	CouponID       string  `json:"coupon_id"`
	Code           string  `json:"code"`
	DiscountType   string  `json:"discount_type"`
	DiscountValue  float64 `json:"discount_value"`
	MinOrderAmount float64 `json:"min_order_amount"`
	IsActive       bool    `json:"is_active"`
	CreatedAt      string  `json:"created_at"`
}

func (r *Repository) ListCouponsAdmin(ctx context.Context) ([]AdminCouponItem, error) {
	if r.db == nil {
		return []AdminCouponItem{}, nil
	}
	rows, err := r.db.Query(ctx, "SELECT id::text, code, discount_type, discount_value, min_order_amount, is_active, created_at FROM coupons ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var coupons []AdminCouponItem
	for rows.Next() {
		var item AdminCouponItem
		var dt time.Time
		if err := rows.Scan(&item.CouponID, &item.Code, &item.DiscountType, &item.DiscountValue, &item.MinOrderAmount, &item.IsActive, &dt); err == nil {
			item.CreatedAt = dt.Format(time.RFC3339)
			coupons = append(coupons, item)
		}
	}
	if coupons == nil { coupons = []AdminCouponItem{} }
	return coupons, nil
}

func (r *Repository) CreateCoupon(ctx context.Context, code, discountType string, discountVal, minOrder float64, userID string) (string, error) {
	if r.db == nil {
		return "mock-coupon-id", nil
	}
	var couponID string
	err := r.db.QueryRow(ctx, `
		INSERT INTO coupons (code, name, discount_type, discount_value, min_order_amount, created_by, is_active)
		VALUES ($1, $1, $2, $3, $4, $5::uuid, true)
		ON CONFLICT (code) DO UPDATE SET discount_value = EXCLUDED.discount_value
		RETURNING id::text`,
		code, discountType, discountVal, minOrder, userID,
	).Scan(&couponID)
	if err != nil {
		return "", err
	}
	return couponID, nil
}
