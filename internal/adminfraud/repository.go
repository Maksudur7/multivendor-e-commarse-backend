package adminfraud

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

type RiskProfileItem struct {
	ProfileID      string  `json:"profile_id"`
	UserID         string  `json:"user_id"`
	Email          string  `json:"email"`
	RiskScore      float64 `json:"risk_score"`
	RiskLevel      string  `json:"risk_level"`
	TotalOrders    int     `json:"total_orders"`
	TotalReturns   int     `json:"total_returns"`
	ReturnRate     float64 `json:"return_rate"`
	CodRefusalRate float64 `json:"cod_refusal_rate"`
	UpdatedAt      string  `json:"updated_at"`
}

func (r *Repository) ListRiskProfiles(ctx context.Context, riskLevel string) ([]RiskProfileItem, error) {
	if r.db == nil {
		return []RiskProfileItem{}, nil
	}
	query := `
		SELECT rp.id::text, rp.user_id::text, rp.risk_score, rp.risk_level,
		       rp.total_orders, rp.total_returns, rp.return_rate,
		       rp.cod_refusal_rate, COALESCE(u.email,''), rp.updated_at
		FROM buyer_risk_profiles rp
		LEFT JOIN users u ON u.id = rp.user_id`
	args := []interface{}{}
	if riskLevel != "" {
		query += " WHERE rp.risk_level = $1"
		args = append(args, riskLevel)
	}
	query += " ORDER BY rp.risk_score DESC LIMIT 100"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []RiskProfileItem
	for rows.Next() {
		var item RiskProfileItem
		var dt time.Time
		if err := rows.Scan(&item.ProfileID, &item.UserID, &item.RiskScore, &item.RiskLevel, &item.TotalOrders, &item.TotalReturns, &item.ReturnRate, &item.CodRefusalRate, &item.Email, &dt); err == nil {
			item.UpdatedAt = dt.Format(time.RFC3339)
			list = append(list, item)
		}
	}
	if list == nil { list = []RiskProfileItem{} }
	return list, nil
}

type BlacklistPhoneItem struct {
	BlacklistID string `json:"blacklist_id"`
	Phone       string `json:"phone"`
	Reason      string `json:"reason"`
	Note        string `json:"note"`
	IsActive    bool   `json:"is_active"`
	CreatedAt   string `json:"created_at"`
}

func (r *Repository) ListBlacklists(ctx context.Context) ([]BlacklistPhoneItem, error) {
	if r.db == nil {
		return []BlacklistPhoneItem{}, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT id::text, phone, reason, COALESCE(note,''), is_active, created_at
		FROM blacklisted_phones
		WHERE is_active = true
		ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []BlacklistPhoneItem
	for rows.Next() {
		var item BlacklistPhoneItem
		var dt time.Time
		if err := rows.Scan(&item.BlacklistID, &item.Phone, &item.Reason, &item.Note, &item.IsActive, &dt); err == nil {
			item.CreatedAt = dt.Format(time.RFC3339)
			list = append(list, item)
		}
	}
	if list == nil { list = []BlacklistPhoneItem{} }
	return list, nil
}

type IPBlockItem struct {
	BlockID      string `json:"block_id"`
	IPAddress    string `json:"ip_address"`
	Reason       string `json:"reason"`
	BlockCount   int    `json:"block_count"`
	BlockedUntil string `json:"blocked_until"`
	CreatedAt    string `json:"created_at"`
}

func (r *Repository) ListIPBlocks(ctx context.Context) ([]IPBlockItem, error) {
	if r.db == nil {
		return []IPBlockItem{}, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT id::text, ip_address::text, COALESCE(reason,''), block_count, blocked_until, created_at
		FROM ip_blocks
		WHERE blocked_until > now()
		ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []IPBlockItem
	for rows.Next() {
		var item IPBlockItem
		var dt1, dt2 time.Time
		if err := rows.Scan(&item.BlockID, &item.IPAddress, &item.Reason, &item.BlockCount, &dt1, &dt2); err == nil {
			item.BlockedUntil = dt1.Format(time.RFC3339)
			item.CreatedAt = dt2.Format(time.RFC3339)
			list = append(list, item)
		}
	}
	if list == nil { list = []IPBlockItem{} }
	return list, nil
}

func (r *Repository) BlacklistPhone(ctx context.Context, phone, reason, note string) (string, error) {
	if r.db == nil {
		return "mock-blacklist-id", nil
	}
	var blacklistID string
	err := r.db.QueryRow(ctx, `
		INSERT INTO blacklisted_phones (phone, reason, note, is_active)
		VALUES ($1, $2, $3, true)
		ON CONFLICT (phone) DO UPDATE SET is_active = true, reason = EXCLUDED.reason
		RETURNING id::text`,
		phone, reason, note,
	).Scan(&blacklistID)
	if err != nil {
		return "", err
	}
	return blacklistID, nil
}

func (r *Repository) BlockIP(ctx context.Context, ip, reason string, blockedUntil time.Time) (string, error) {
	if r.db == nil {
		return "mock-block-id", nil
	}
	var blockID string
	err := r.db.QueryRow(ctx, `
		INSERT INTO ip_blocks (ip_address, reason, blocked_until)
		VALUES ($1::inet, $2, $3)
		RETURNING id::text`,
		ip, reason, blockedUntil,
	).Scan(&blockID)
	if err != nil {
		return "", err
	}
	return blockID, nil
}

func (r *Repository) RecalculateRisk(ctx context.Context, userID string) error {
	if r.db == nil { return nil }
	var totalOrders, totalReturns int
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM master_orders WHERE user_id::text = $1`, userID).Scan(&totalOrders)

	returnRate := 0.0
	if totalOrders > 0 {
		returnRate = float64(totalReturns) / float64(totalOrders) * 100
	}

	riskScore := returnRate * 0.5
	riskLevel := "LOW"
	if riskScore >= 20 { riskLevel = "MEDIUM" }
	if riskScore >= 40 { riskLevel = "HIGH" }
	if riskScore >= 60 { riskLevel = "VERY_HIGH" }

	_, err := r.db.Exec(ctx, `
		INSERT INTO buyer_risk_profiles (user_id, risk_score, risk_level, total_orders, last_recalculated_at)
		VALUES ($1::uuid, $2, $3, $4, now())
		ON CONFLICT (user_id) DO UPDATE SET
			risk_score = EXCLUDED.risk_score,
			risk_level = EXCLUDED.risk_level,
			total_orders = EXCLUDED.total_orders,
			last_recalculated_at = now()`,
		userID, riskScore, riskLevel, totalOrders)
	return err
}

func (r *Repository) UpdateRiskProfile(ctx context.Context, profileID, riskLevel, note string, score float64) error {
	if r.db == nil { return nil }
	res, err := r.db.Exec(ctx, `
		UPDATE buyer_risk_profiles SET
			risk_level = COALESCE(NULLIF($1,''), risk_level),
			admin_note = COALESCE(NULLIF($2,''), admin_note),
			risk_score = CASE WHEN $3 > 0 THEN $3 ELSE risk_score END,
			manually_reviewed = true,
			updated_at = now()
		WHERE id::text = $4`,
		riskLevel, note, score, profileID)
	if err != nil { return err }
	if res.RowsAffected() == 0 { return fmt.Errorf("profile not found") }
	return nil
}

func (r *Repository) UnblockIP(ctx context.Context, blockID string) error {
	if r.db == nil { return nil }
	res, err := r.db.Exec(ctx, "DELETE FROM ip_blocks WHERE id::text = $1", blockID)
	if err != nil { return err }
	if res.RowsAffected() == 0 { return fmt.Errorf("ip block not found") }
	return nil
}
