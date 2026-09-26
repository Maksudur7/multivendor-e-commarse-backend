package affiliate

import (
	"context"
	"errors"
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

type AffiliateProfile struct {
	Registered   bool    `json:"registered"`
	AffiliateID  string  `json:"affiliate_id,omitempty"`
	UserID       string  `json:"user_id"`
	Status       string  `json:"status,omitempty"`
	ReferralCode string  `json:"referral_code,omitempty"`
	TotalClicks  int64   `json:"total_clicks"`
	TotalEarned  float64 `json:"total_earned"`
	CreatedAt    string  `json:"created_at,omitempty"`
}

func (r *Repository) GetProfileByUserID(ctx context.Context, userID string) (*AffiliateProfile, error) {
	if r.db == nil {
		return &AffiliateProfile{Registered: true, UserID: userID, Status: "PENDING"}, nil
	}
	var ap AffiliateProfile
	var dt time.Time
	err := r.db.QueryRow(ctx, `
		SELECT id::text, status, referral_code, total_clicks, total_earned, created_at
		FROM affiliate_profiles WHERE user_id::text = $1`, userID).
		Scan(&ap.AffiliateID, &ap.Status, &ap.ReferralCode, &ap.TotalClicks, &ap.TotalEarned, &dt)
	if err != nil {
		return &AffiliateProfile{Registered: false, UserID: userID}, nil
	}
	ap.Registered = true
	ap.UserID = userID
	ap.CreatedAt = dt.Format(time.RFC3339)
	return &ap, nil
}

type AffiliateLinkItem struct {
	LinkID       string `json:"link_id"`
	ShortCode    string `json:"short_code"`
	FullURL      string `json:"full_url"`
	TotalClicks  int64  `json:"total_clicks"`
	IsActive     bool   `json:"is_active"`
	ProductTitle string `json:"product_title"`
	CreatedAt    string `json:"created_at"`
}

func (r *Repository) GetLinksByUserID(ctx context.Context, userID string) ([]AffiliateLinkItem, error) {
	if r.db == nil {
		return []AffiliateLinkItem{}, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT al.id::text, al.short_code, al.full_url, al.total_clicks, al.is_active, al.created_at,
		       COALESCE(p.name, 'General Link') as product_title
		FROM affiliate_links_simple al
		LEFT JOIN products p ON p.id = al.product_id
		JOIN affiliate_profiles ap ON ap.id = al.affiliate_id AND ap.user_id::text = $1
		ORDER BY al.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []AffiliateLinkItem
	for rows.Next() {
		var item AffiliateLinkItem
		var dt time.Time
		if err := rows.Scan(&item.LinkID, &item.ShortCode, &item.FullURL, &item.TotalClicks, &item.IsActive, &dt, &item.ProductTitle); err == nil {
			item.CreatedAt = dt.Format(time.RFC3339)
			links = append(links, item)
		}
	}
	if links == nil { links = []AffiliateLinkItem{} }
	return links, nil
}

func (r *Repository) GetClicks(ctx context.Context, userID string) (int64, error) {
	if r.db == nil { return 0, nil }
	var totalClicks int64
	r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(al.total_clicks),0)
		FROM affiliate_links_simple al
		JOIN affiliate_profiles ap ON ap.id = al.affiliate_id AND ap.user_id::text = $1`, userID).Scan(&totalClicks)
	return totalClicks, nil
}

func (r *Repository) GetEarnings(ctx context.Context, userID string) (float64, error) {
	if r.db == nil { return 0.0, nil }
	var totalEarned float64
	r.db.QueryRow(ctx, `SELECT COALESCE(total_earned,0) FROM affiliate_profiles WHERE user_id::text = $1`, userID).Scan(&totalEarned)
	return totalEarned, nil
}

func (r *Repository) Apply(ctx context.Context, userID string) (string, string, error) {
	if r.db == nil {
		return "", "", errors.New("database connection unavailable")
	}
	var existingID string
	err := r.db.QueryRow(ctx, "SELECT id::text FROM affiliate_profiles WHERE user_id::text = $1", userID).Scan(&existingID)
	if err == nil {
		return "", "", fmt.Errorf("already applied")
	}

	refCode := fmt.Sprintf("AFF%s%d", userID[:6], time.Now().UnixNano()%1000)
	var profileID string
	// Try with referral_code first; if column missing, insert without it
	err = r.db.QueryRow(ctx, `
		INSERT INTO affiliate_profiles (user_id, status, referral_code)
		VALUES ($1::uuid, 'APPROVED', $2)
		ON CONFLICT (user_id) DO UPDATE SET status = 'APPROVED'
		RETURNING id::text`, userID, refCode).Scan(&profileID)
	if err != nil {
		// Fallback: insert without referral_code (column may not exist yet)
		err2 := r.db.QueryRow(ctx, `
			INSERT INTO affiliate_profiles (user_id, status)
			VALUES ($1::uuid, 'APPROVED')
			ON CONFLICT (user_id) DO UPDATE SET status = 'APPROVED'
			RETURNING id::text`, userID).Scan(&profileID)
		if err2 != nil {
			return "", "", err2
		}
	}
	return profileID, refCode, nil
}

func (r *Repository) ApproveAffiliate(ctx context.Context, userID string) error {
	if r.db == nil {
		return errors.New("database connection unavailable")
	}
	_, err := r.db.Exec(ctx,
		`UPDATE affiliate_profiles SET status = 'APPROVED', updated_at = now() WHERE user_id::text = $1`,
		userID)
	return err
}

func (r *Repository) CreateLink(ctx context.Context, userID, productID, fullURL string) (string, string, error) {
	if r.db == nil {
		return "", "", errors.New("database connection unavailable")
	}
	var affiliateID string
	err := r.db.QueryRow(ctx, "SELECT id::text FROM affiliate_profiles WHERE user_id::text = $1 AND status = 'APPROVED'", userID).Scan(&affiliateID)
	if err != nil {
		return "", "", fmt.Errorf("affiliate profile not approved")
	}

	shortCode := fmt.Sprintf("AFF-%s-%d", userID[:6], time.Now().UnixNano()%10000)
	var linkID string
	var pID *string
	if productID != "" { pID = &productID }
	err = r.db.QueryRow(ctx, `
		INSERT INTO affiliate_links_simple (affiliate_id, product_id, short_code, full_url)
		VALUES ($1::uuid, $2, $3, $4)
		RETURNING id::text`,
		affiliateID, pID, shortCode, fullURL,
	).Scan(&linkID)
	if err != nil {
		return "", "", err
	}
	return linkID, shortCode, nil
}

func (r *Repository) RequestWithdrawal(ctx context.Context, userID string, amount float64) (string, error) {
	if r.db == nil {
		return "", errors.New("database connection unavailable")
	}
	var affiliateID string
	var totalEarned float64
	err := r.db.QueryRow(ctx, "SELECT id::text, total_earned FROM affiliate_profiles WHERE user_id::text = $1 AND status = 'APPROVED'", userID).Scan(&affiliateID, &totalEarned)
	if err != nil {
		return "", fmt.Errorf("affiliate profile not found or not approved")
	}
	if amount > totalEarned {
		return "", fmt.Errorf("insufficient balance: %.2f BDT", totalEarned)
	}

	var withdrawalID string
	err = r.db.QueryRow(ctx, `
		INSERT INTO affiliate_withdrawals (affiliate_id, user_id, amount, status)
		VALUES ($1::uuid, $2::uuid, $3, 'PENDING') RETURNING id::text`,
		affiliateID, userID, amount,
	).Scan(&withdrawalID)
	if err != nil {
		return "", err
	}
	return withdrawalID, nil
}

func (r *Repository) UpdateProfile(ctx context.Context, userID string) error {
	if r.db == nil { return nil }
	_, err := r.db.Exec(ctx, "UPDATE affiliate_profiles SET updated_at = now() WHERE user_id::text = $1", userID)
	return err
}

func (r *Repository) DeleteLink(ctx context.Context, linkID, userID string) error {
	if r.db == nil { return nil }
	_, err := r.db.Exec(ctx, `
		DELETE FROM affiliate_links_simple al
		USING affiliate_profiles ap
		WHERE al.id::text = $1 AND al.affiliate_id = ap.id AND ap.user_id::text = $2`,
		linkID, userID)
	return err
}

type AffiliateAdminApplicationItem struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Status    string    `json:"status"`
	RefCode   string    `json:"referral_code"`
	CreatedAt time.Time `json:"created_at"`
}

func (r *Repository) ListApplicationsAdmin(ctx context.Context, status string) ([]AffiliateAdminApplicationItem, error) {
	if r.db == nil {
		return []AffiliateAdminApplicationItem{}, nil
	}
	whereClause := ""
	args := []interface{}{}
	if status != "" {
		whereClause = " WHERE status = $1"
		args = append(args, status)
	}

	rows, err := r.db.Query(ctx, "SELECT id::text, user_id::text, status, referral_code, created_at FROM affiliate_profiles"+whereClause+" ORDER BY created_at DESC", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []AffiliateAdminApplicationItem{}
	for rows.Next() {
		var item AffiliateAdminApplicationItem
		if err := rows.Scan(&item.ID, &item.UserID, &item.Status, &item.RefCode, &item.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, nil
}

func (r *Repository) ReviewApplicationAdmin(ctx context.Context, profileID, status string) (int64, error) {
	if r.db == nil {
		return 0, nil
	}
	var userID string
	err := r.db.QueryRow(ctx, "UPDATE affiliate_profiles SET status = $1, updated_at = now() WHERE id::text = $2 RETURNING user_id::text", status, profileID).Scan(&userID)
	if err != nil {
		return 0, err
	}
	if (status == "APPROVED" || status == "ACTIVE") && userID != "" {
		_, _ = r.db.Exec(ctx, "UPDATE users SET role = 'AFFILIATE', updated_at = now() WHERE id::text = $1", userID)
	}
	return 1, nil
}

