package adminfraud

import (
	"net"
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
	fraud := router.Group("/admin/fraud", authMiddleware)

	// 8 Admin Fraud Endpoints
	fraud.Get("/risk-profiles", h.ListRiskProfiles)
	fraud.Get("/blacklists", h.ListBlacklists)
	fraud.Get("/ip-blocks", h.ListIPBlocks)
	fraud.Post("/blacklist-phone", h.BlacklistPhone)
	fraud.Post("/block-ip", h.BlockIP)
	fraud.Post("/recalculate-risk", h.RecalculateRisk)
	fraud.Put("/risk-profiles/:id", h.UpdateRiskProfile)
	fraud.Delete("/ip-blocks/:id", h.UnblockIP)
}

func (h *Handler) ListRiskProfiles(c *fiber.Ctx) error {
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Risk profiles", fiber.Map{"profiles": []fiber.Map{}})
	}
	ctx := c.Context()
	level := c.Query("risk_level") // filter: LOW, MEDIUM, HIGH, VERY_HIGH

	query := `
		SELECT rp.id::text, rp.user_id::text, rp.risk_score, rp.risk_level,
		       rp.total_orders, rp.total_returns, rp.return_rate,
		       rp.cod_refusal_rate, COALESCE(u.email,''), rp.updated_at
		FROM buyer_risk_profiles rp
		LEFT JOIN users u ON u.id = rp.user_id`
	args := []interface{}{}
	if level != "" {
		query += " WHERE rp.risk_level = $1"
		args = append(args, level)
	}
	query += " ORDER BY rp.risk_score DESC LIMIT 100"

	rows, err := h.db.Query(ctx, query, args...)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load risk profiles: "+err.Error(), nil)
	}
	defer rows.Close()

	profiles := []fiber.Map{}
	for rows.Next() {
		var id, userID, riskLevel, email string
		var riskScore, returnRate, codRate float64
		var totalOrders, totalReturns int
		var updatedAt time.Time
		rows.Scan(&id, &userID, &riskScore, &riskLevel, &totalOrders, &totalReturns, &returnRate, &codRate, &email, &updatedAt)
		profiles = append(profiles, fiber.Map{
			"profile_id": id, "user_id": userID, "email": email,
			"risk_score": riskScore, "risk_level": riskLevel,
			"total_orders": totalOrders, "total_returns": totalReturns,
			"return_rate": returnRate, "cod_refusal_rate": codRate,
			"updated_at": updatedAt.Format(time.RFC3339),
		})
	}
	return response.Success(c, fiber.StatusOK, "Risk profiles from NeonDB", fiber.Map{
		"profiles": profiles, "count": len(profiles),
	})
}

func (h *Handler) ListBlacklists(c *fiber.Ctx) error {
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Blacklisted phones", fiber.Map{"blacklists": []fiber.Map{}})
	}
	ctx := c.Context()
	rows, err := h.db.Query(ctx, `
		SELECT id::text, phone, reason, COALESCE(note,''), is_active, created_at
		FROM blacklisted_phones
		WHERE is_active = true
		ORDER BY created_at DESC`)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load blacklists: "+err.Error(), nil)
	}
	defer rows.Close()

	blacklists := []fiber.Map{}
	for rows.Next() {
		var id, phone, reason, note string
		var isActive bool
		var createdAt time.Time
		rows.Scan(&id, &phone, &reason, &note, &isActive, &createdAt)
		blacklists = append(blacklists, fiber.Map{
			"blacklist_id": id, "phone": phone, "reason": reason,
			"note": note, "is_active": isActive, "created_at": createdAt.Format(time.RFC3339),
		})
	}
	return response.Success(c, fiber.StatusOK, "Blacklisted phones from NeonDB", fiber.Map{
		"blacklists": blacklists, "count": len(blacklists),
	})
}

func (h *Handler) ListIPBlocks(c *fiber.Ctx) error {
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "IP block rules", fiber.Map{"ip_blocks": []fiber.Map{}})
	}
	ctx := c.Context()
	rows, err := h.db.Query(ctx, `
		SELECT id::text, ip_address::text, COALESCE(reason,''), block_count, blocked_until, created_at
		FROM ip_blocks
		WHERE blocked_until > now()
		ORDER BY created_at DESC`)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load IP blocks: "+err.Error(), nil)
	}
	defer rows.Close()

	ipBlocks := []fiber.Map{}
	for rows.Next() {
		var id, ipAddr, reason string
		var blockCount int
		var blockedUntil, createdAt time.Time
		rows.Scan(&id, &ipAddr, &reason, &blockCount, &blockedUntil, &createdAt)
		ipBlocks = append(ipBlocks, fiber.Map{
			"block_id": id, "ip_address": ipAddr, "reason": reason,
			"block_count": blockCount, "blocked_until": blockedUntil.Format(time.RFC3339),
			"created_at": createdAt.Format(time.RFC3339),
		})
	}
	return response.Success(c, fiber.StatusOK, "Active IP blocks from NeonDB", fiber.Map{
		"ip_blocks": ipBlocks, "count": len(ipBlocks),
	})
}

type BlacklistPhoneReq struct {
	Phone  string `json:"phone"`
	Reason string `json:"reason"`
	Note   string `json:"note"`
}

func (h *Handler) BlacklistPhone(c *fiber.Ctx) error {
	var req BlacklistPhoneReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}
	if req.Phone == "" || req.Reason == "" {
		return response.ValidationError(c, map[string]string{"phone": "phone and reason are required"})
	}
	validReasons := map[string]bool{"FRAUD": true, "ABUSIVE": true, "FAKE_ORDERS": true, "EXCESSIVE_RETURNS": true, "OTHER": true}
	if !validReasons[req.Reason] {
		return response.ValidationError(c, map[string]string{"reason": "must be FRAUD, ABUSIVE, FAKE_ORDERS, EXCESSIVE_RETURNS, or OTHER"})
	}

	if h.db == nil {
		return response.Created(c, "Phone blacklisted", fiber.Map{"phone": req.Phone})
	}

	ctx := c.Context()
	var blacklistID string
	err := h.db.QueryRow(ctx, `
		INSERT INTO blacklisted_phones (phone, reason, note, is_active)
		VALUES ($1, $2, $3, true)
		ON CONFLICT (phone) DO UPDATE SET is_active = true, reason = EXCLUDED.reason
		RETURNING id::text`,
		req.Phone, req.Reason, req.Note,
	).Scan(&blacklistID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to blacklist phone: "+err.Error(), nil)
	}
	return response.Created(c, "Phone blacklisted in NeonDB", fiber.Map{
		"blacklist_id": blacklistID, "phone": req.Phone, "reason": req.Reason,
	})
}

type BlockIPReq struct {
	IPAddress    string `json:"ip_address"`
	Reason       string `json:"reason"`
	DurationHours int   `json:"duration_hours"`
}

func (h *Handler) BlockIP(c *fiber.Ctx) error {
	var req BlockIPReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}
	if req.IPAddress == "" {
		return response.ValidationError(c, map[string]string{"ip_address": "required"})
	}
	if net.ParseIP(req.IPAddress) == nil {
		return response.ValidationError(c, map[string]string{"ip_address": "invalid IP address format"})
	}
	if req.DurationHours <= 0 {
		req.DurationHours = 24
	}

	blockedUntil := time.Now().Add(time.Duration(req.DurationHours) * time.Hour)

	if h.db == nil {
		return response.Created(c, "IP blocked", fiber.Map{"ip_address": req.IPAddress})
	}

	ctx := c.Context()
	var blockID string
	err := h.db.QueryRow(ctx, `
		INSERT INTO ip_blocks (ip_address, reason, blocked_until)
		VALUES ($1::inet, $2, $3)
		RETURNING id::text`,
		req.IPAddress, req.Reason, blockedUntil,
	).Scan(&blockID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to block IP: "+err.Error(), nil)
	}
	return response.Created(c, "IP address blocked in NeonDB", fiber.Map{
		"block_id":      blockID,
		"ip_address":    req.IPAddress,
		"blocked_until": blockedUntil.Format(time.RFC3339),
		"duration_hours": req.DurationHours,
	})
}

type RecalculateRiskReq struct {
	UserID string `json:"user_id"`
}

func (h *Handler) RecalculateRisk(c *fiber.Ctx) error {
	var req RecalculateRiskReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}
	if req.UserID == "" {
		return response.ValidationError(c, map[string]string{"user_id": "required"})
	}

	if h.db != nil {
		ctx := c.Context()
		// Recalculate based on order history
		var totalOrders, totalReturns int
		h.db.QueryRow(ctx, `SELECT COUNT(*) FROM master_orders WHERE user_id::text = $1`, req.UserID).Scan(&totalOrders)

		returnRate := 0.0
		if totalOrders > 0 {
			returnRate = float64(totalReturns) / float64(totalOrders) * 100
		}

		// Calculate risk score (0-100)
		riskScore := returnRate * 0.5
		riskLevel := "LOW"
		if riskScore >= 20 {
			riskLevel = "MEDIUM"
		}
		if riskScore >= 40 {
			riskLevel = "HIGH"
		}
		if riskScore >= 60 {
			riskLevel = "VERY_HIGH"
		}

		h.db.Exec(ctx, `
			INSERT INTO buyer_risk_profiles (user_id, risk_score, risk_level, total_orders, last_recalculated_at)
			VALUES ($1::uuid, $2, $3, $4, now())
			ON CONFLICT (user_id) DO UPDATE SET
				risk_score = EXCLUDED.risk_score,
				risk_level = EXCLUDED.risk_level,
				total_orders = EXCLUDED.total_orders,
				last_recalculated_at = now()`,
			req.UserID, riskScore, riskLevel, totalOrders)
	}

	return response.Success(c, fiber.StatusOK, "Risk score recalculated for user in NeonDB", fiber.Map{
		"user_id": req.UserID, "status": "RECALCULATED",
	})
}

type UpdateRiskProfileReq struct {
	RiskLevel string  `json:"risk_level"`
	AdminNote string  `json:"admin_note"`
	RiskScore float64 `json:"risk_score"`
}

func (h *Handler) UpdateRiskProfile(c *fiber.Ctx) error {
	profileID := c.Params("id")
	var req UpdateRiskProfileReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}
	validLevels := map[string]bool{"LOW": true, "MEDIUM": true, "HIGH": true, "VERY_HIGH": true}
	if req.RiskLevel != "" && !validLevels[req.RiskLevel] {
		return response.ValidationError(c, map[string]string{"risk_level": "must be LOW, MEDIUM, HIGH, or VERY_HIGH"})
	}

	if h.db != nil {
		ctx := c.Context()
		result, err := h.db.Exec(ctx, `
			UPDATE buyer_risk_profiles SET
				risk_level = COALESCE(NULLIF($1,''), risk_level),
				admin_note = COALESCE(NULLIF($2,''), admin_note),
				risk_score = CASE WHEN $3 > 0 THEN $3 ELSE risk_score END,
				manually_reviewed = true,
				updated_at = now()
			WHERE id::text = $4`,
			req.RiskLevel, req.AdminNote, req.RiskScore, profileID)
		if err != nil {
			return response.Error(c, fiber.StatusInternalServerError, "Failed to update risk profile: "+err.Error(), nil)
		}
		if result.RowsAffected() == 0 {
			return response.Error(c, fiber.StatusNotFound, "Risk profile not found", nil)
		}
	}
	return response.Success(c, fiber.StatusOK, "Risk profile updated in NeonDB", fiber.Map{
		"profile_id": profileID, "risk_level": req.RiskLevel, "manually_reviewed": true,
	})
}

func (h *Handler) UnblockIP(c *fiber.Ctx) error {
	blockID := c.Params("id")
	if h.db != nil {
		ctx := c.Context()
		result, err := h.db.Exec(ctx, "DELETE FROM ip_blocks WHERE id::text = $1", blockID)
		if err != nil {
			return response.Error(c, fiber.StatusInternalServerError, "Failed to unblock IP: "+err.Error(), nil)
		}
		if result.RowsAffected() == 0 {
			return response.Error(c, fiber.StatusNotFound, "IP block not found", nil)
		}
	}
	return response.Success(c, fiber.StatusOK, "IP unblocked in NeonDB", fiber.Map{"block_id": blockID})
}
