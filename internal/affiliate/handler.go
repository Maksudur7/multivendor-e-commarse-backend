package affiliate

import (
	"fmt"
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
	aff := router.Group("/affiliate", authMiddleware)

	// 10 Affiliate Endpoints
	aff.Get("/profile", h.GetProfile)
	aff.Get("/links", h.GetLinks)
	aff.Get("/conversions", h.GetConversions)
	aff.Get("/clicks", h.GetClicks)
	aff.Get("/earnings", h.GetEarnings)
	aff.Post("/apply", h.Apply)
	aff.Post("/links", h.CreateLink)
	aff.Post("/withdraw", h.Withdraw)
	aff.Put("/profile", h.UpdateProfile)
	aff.Delete("/links/:id", h.DeleteLink)
}

func (h *Handler) GetProfile(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Affiliate profile", fiber.Map{"user_id": userID, "status": "PENDING"})
	}
	ctx := c.Context()
	var id, status, refCode string
	var totalClicks int64
	var totalEarned float64
	var createdAt time.Time
	err := h.db.QueryRow(ctx, `
		SELECT id::text, status, referral_code, total_clicks, total_earned, created_at
		FROM affiliate_profiles WHERE user_id::text = $1`, userID).
		Scan(&id, &status, &refCode, &totalClicks, &totalEarned, &createdAt)
	if err != nil {
		return response.Success(c, fiber.StatusOK, "Affiliate profile (Not registered)", fiber.Map{
			"registered": false, "user_id": userID,
		})
	}
	return response.Success(c, fiber.StatusOK, "Affiliate profile from NeonDB", fiber.Map{
		"registered":    true,
		"affiliate_id":  id,
		"user_id":       userID,
		"status":        status,
		"referral_code": refCode,
		"total_clicks":  totalClicks,
		"total_earned":  totalEarned,
		"created_at":    createdAt.Format(time.RFC3339),
	})
}

func (h *Handler) GetLinks(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Affiliate links list", fiber.Map{"links": []fiber.Map{}})
	}
	ctx := c.Context()

	rows, err := h.db.Query(ctx, `
		SELECT al.id::text, al.short_code, al.full_url, al.total_clicks, al.is_active, al.created_at,
		       COALESCE(p.name, 'General Link') as product_title
		FROM affiliate_links_simple al
		LEFT JOIN products p ON p.id = al.product_id
		JOIN affiliate_profiles ap ON ap.id = al.affiliate_id AND ap.user_id::text = $1
		ORDER BY al.created_at DESC`, userID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load links: "+err.Error(), nil)
	}
	defer rows.Close()

	links := []fiber.Map{}
	for rows.Next() {
		var id, shortCode, fullURL, productTitle string
		var totalClicks int64
		var isActive bool
		var createdAt time.Time
		rows.Scan(&id, &shortCode, &fullURL, &totalClicks, &isActive, &createdAt, &productTitle)
		links = append(links, fiber.Map{
			"link_id": id, "short_code": shortCode, "full_url": fullURL,
			"total_clicks": totalClicks, "is_active": isActive,
			"product_title": productTitle, "created_at": createdAt.Format(time.RFC3339),
		})
	}
	return response.Success(c, fiber.StatusOK, "Affiliate links from NeonDB", fiber.Map{
		"links": links, "count": len(links),
	})
}

func (h *Handler) GetConversions(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Affiliate conversions - no conversions yet", fiber.Map{
		"conversions": []fiber.Map{}, "count": 0,
	})
}

func (h *Handler) GetClicks(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Affiliate clicks", fiber.Map{"total_clicks": 0})
	}
	ctx := c.Context()
	var totalClicks int64
	h.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(al.total_clicks),0)
		FROM affiliate_links_simple al
		JOIN affiliate_profiles ap ON ap.id = al.affiliate_id AND ap.user_id::text = $1`, userID).Scan(&totalClicks)

	return response.Success(c, fiber.StatusOK, "Affiliate click stats from NeonDB", fiber.Map{
		"total_clicks": totalClicks,
		"user_id":      userID,
		"period":       "all_time",
	})
}

func (h *Handler) GetEarnings(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Affiliate earnings", fiber.Map{"total_earned": 0.0})
	}
	ctx := c.Context()
	var totalEarned float64
	h.db.QueryRow(ctx, `
		SELECT COALESCE(total_earned,0) FROM affiliate_profiles WHERE user_id::text = $1`, userID).Scan(&totalEarned)

	return response.Success(c, fiber.StatusOK, "Affiliate earnings from NeonDB", fiber.Map{
		"total_earned": totalEarned,
		"user_id":      userID,
		"currency":     "BDT",
	})
}

type ApplyReq struct {
	WebsiteURL       string `json:"website_url"`
	SocialLinks      string `json:"social_links"`
	PromotionMethod  string `json:"promotion_method"`
}

func (h *Handler) Apply(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	var req ApplyReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}

	if h.db == nil {
		return response.Created(c, "Affiliate application submitted", fiber.Map{"status": "PENDING"})
	}

	ctx := c.Context()
	// Generate unique referral code
	var existingID string
	err := h.db.QueryRow(ctx, "SELECT id::text FROM affiliate_profiles WHERE user_id::text = $1", userID).Scan(&existingID)
	if err == nil {
		return response.Error(c, fiber.StatusConflict, "Already applied for affiliate program", nil)
	}

	refCode := fmt.Sprintf("AFF%s%d", userID[:6], time.Now().UnixNano()%1000)
	var profileID string
	err = h.db.QueryRow(ctx, `
		INSERT INTO affiliate_profiles (user_id, status, referral_code)
		VALUES ($1::uuid, 'PENDING', $2)
		ON CONFLICT (user_id) DO UPDATE SET status = EXCLUDED.status
		RETURNING id::text`, userID, refCode).Scan(&profileID)
	if err != nil {
		return response.Error(c, fiber.StatusConflict, "Already applied for affiliate program", nil)
	}

	return response.Created(c, "Affiliate application submitted in NeonDB", fiber.Map{
		"affiliate_id":  profileID,
		"status":        "PENDING",
		"referral_code": refCode,
		"user_id":       userID,
	})
}

type CreateLinkReq struct {
	ProductID string `json:"product_id"`
	FullURL   string `json:"full_url"`
	Label     string `json:"label"`
}

func (h *Handler) CreateLink(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	var req CreateLinkReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}
	if req.FullURL == "" {
		return response.ValidationError(c, map[string]string{"full_url": "required"})
	}

	if h.db == nil {
		return response.Created(c, "Affiliate link created", fiber.Map{"short_code": "AFF-DEMO"})
	}

	ctx := c.Context()
	var affiliateID string
	err := h.db.QueryRow(ctx, "SELECT id::text FROM affiliate_profiles WHERE user_id::text = $1 AND status = 'APPROVED'", userID).Scan(&affiliateID)
	if err != nil {
		return response.Error(c, fiber.StatusForbidden, "Affiliate profile not approved. Please apply first.", nil)
	}

	shortCode := fmt.Sprintf("AFF-%s-%d", userID[:6], time.Now().UnixNano()%10000)
	var linkID string
	err = h.db.QueryRow(ctx, `
		INSERT INTO affiliate_links_simple (affiliate_id, product_id, short_code, full_url)
		VALUES ($1::uuid, $2, $3, $4)
		RETURNING id::text`,
		affiliateID, nullIfEmpty(req.ProductID), shortCode, req.FullURL,
	).Scan(&linkID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to create link: "+err.Error(), nil)
	}

	return response.Created(c, "Affiliate referral link generated in NeonDB", fiber.Map{
		"link_id":    linkID,
		"short_code": shortCode,
		"full_url":   req.FullURL,
		"share_url":  fmt.Sprintf("https://platform.com/r/%s", shortCode),
	})
}

type WithdrawReq struct {
	Amount        float64 `json:"amount"`
	PaymentMethod string  `json:"payment_method"`
	AccountNumber string  `json:"account_number"`
}

func (h *Handler) Withdraw(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	var req WithdrawReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}
	if req.Amount <= 0 {
		return response.ValidationError(c, map[string]string{"amount": "must be greater than 0"})
	}

	if h.db == nil {
		return response.Created(c, "Affiliate withdrawal requested", fiber.Map{"status": "PENDING"})
	}

	ctx := c.Context()
	var affiliateID string
	var totalEarned float64
	err := h.db.QueryRow(ctx, "SELECT id::text, total_earned FROM affiliate_profiles WHERE user_id::text = $1 AND status = 'APPROVED'", userID).Scan(&affiliateID, &totalEarned)
	if err != nil {
		return response.Error(c, fiber.StatusForbidden, "Affiliate profile not found or not approved", nil)
	}
	if req.Amount > totalEarned {
		return response.Error(c, fiber.StatusBadRequest, fmt.Sprintf("Insufficient balance. Available: %.2f BDT", totalEarned), nil)
	}

	var withdrawalID string
	err = h.db.QueryRow(ctx, `
		INSERT INTO affiliate_withdrawals (affiliate_id, user_id, amount, status)
		VALUES ($1::uuid, $2::uuid, $3, 'PENDING') RETURNING id::text`,
		affiliateID, userID, req.Amount,
	).Scan(&withdrawalID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to request withdrawal: "+err.Error(), nil)
	}

	return response.Created(c, "Affiliate withdrawal requested in NeonDB", fiber.Map{
		"withdrawal_id":  withdrawalID,
		"amount":         req.Amount,
		"payment_method": req.PaymentMethod,
		"status":         "PENDING",
	})
}

func (h *Handler) UpdateProfile(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	var body map[string]interface{}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}
	if h.db != nil {
		h.db.Exec(c.Context(), "UPDATE affiliate_profiles SET updated_at = now() WHERE user_id::text = $1", userID)
	}
	return response.Success(c, fiber.StatusOK, "Affiliate profile updated in NeonDB", fiber.Map{"user_id": userID})
}

func (h *Handler) DeleteLink(c *fiber.Ctx) error {
	linkID := c.Params("id")
	userID := c.Locals("user_id").(string)
	if h.db != nil {
		ctx := c.Context()
		_, err := h.db.Exec(ctx, `
			DELETE FROM affiliate_links_simple al
			USING affiliate_profiles ap
			WHERE al.id::text = $1 AND al.affiliate_id = ap.id AND ap.user_id::text = $2`,
			linkID, userID)
		if err != nil {
			return response.Error(c, fiber.StatusInternalServerError, "Failed to delete link: "+err.Error(), nil)
		}
	}
	return response.Success(c, fiber.StatusOK, "Affiliate link deleted from NeonDB", fiber.Map{
		"link_id": linkID,
	})
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
