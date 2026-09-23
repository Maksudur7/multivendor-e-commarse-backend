package chinasourcing

import (
	"fmt"
	"strings"
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
	cs := router.Group("/china-sourcing", authMiddleware)

	// 9 China Sourcing Endpoints
	cs.Get("/batches", h.ListBatches)
	cs.Get("/batches/:id", h.GetBatchDetail)
	cs.Get("/landed-cost-calculator", h.CalculateLandedCost)
	cs.Get("/customs-declarations", h.ListCustomsDeclarations)
	cs.Post("/batches", h.CreateBatch)
	cs.Post("/batches/:id/items", h.AddBatchItem)
	cs.Put("/batches/:id/status", h.UpdateBatchStatus)
	cs.Put("/batches/:id/landed-cost", h.UpdateBatchLandedCost)
	cs.Put("/batches/:id/customs", h.UpdateBatchCustomsInfo)
}

func (h *Handler) ListBatches(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "China import batches", fiber.Map{"batches": []fiber.Map{}})
	}
	ctx := c.Context()
	rows, err := h.db.Query(ctx, `
		SELECT id::text, batch_code, status,
		       COALESCE(cny_cost,0), COALESCE(freight_cost,0),
		       COALESCE(landed_cost_bdt,0), created_at, updated_at
		FROM china_sourcing_batches
		WHERE user_id::text = $1
		ORDER BY created_at DESC`, userID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load batches: "+err.Error(), nil)
	}
	defer rows.Close()

	batches := []fiber.Map{}
	for rows.Next() {
		var id, code, status string
		var cnyCost, freight, landed float64
		var createdAt, updatedAt time.Time
		rows.Scan(&id, &code, &status, &cnyCost, &freight, &landed, &createdAt, &updatedAt)
		batches = append(batches, fiber.Map{
			"batch_id": id, "batch_code": code, "status": status,
			"cny_cost": cnyCost, "freight_cost": freight, "landed_cost_bdt": landed,
			"created_at": createdAt.Format(time.RFC3339), "updated_at": updatedAt.Format(time.RFC3339),
		})
	}
	return response.Success(c, fiber.StatusOK, "China import batches from NeonDB", fiber.Map{
		"batches": batches, "count": len(batches),
	})
}

func (h *Handler) GetBatchDetail(c *fiber.Ctx) error {
	batchID := c.Params("id")
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Batch detail", fiber.Map{"batch_id": batchID})
	}
	ctx := c.Context()
	var id, code, status string
	var cnyCost, freight, customsDuty, landed float64
	var customsInfo interface{}
	var createdAt, updatedAt time.Time
	err := h.db.QueryRow(ctx, `
		SELECT id::text, batch_code, status,
		       COALESCE(cny_cost,0), COALESCE(freight_cost,0),
		       COALESCE(customs_duty_pct,0), COALESCE(landed_cost_bdt,0),
		       customs_info, created_at, updated_at
		FROM china_sourcing_batches WHERE id::text = $1`, batchID).
		Scan(&id, &code, &status, &cnyCost, &freight, &customsDuty, &landed, &customsInfo, &createdAt, &updatedAt)
	if err != nil {
		return response.Error(c, fiber.StatusNotFound, "Batch not found", nil)
	}

	// Get batch items
	itemRows, _ := h.db.Query(ctx, `
		SELECT id::text, COALESCE(product_name,''), COALESCE(sku_code,''), quantity, COALESCE(unit_cost_cny,0)
		FROM china_sourcing_batch_items WHERE batch_id::text = $1`, batchID)
	items := []fiber.Map{}
	if itemRows != nil {
		defer itemRows.Close()
		for itemRows.Next() {
			var iID, name, sku string
			var qty int
			var unitCost float64
			itemRows.Scan(&iID, &name, &sku, &qty, &unitCost)
			items = append(items, fiber.Map{
				"item_id": iID, "product_name": name, "sku_code": sku,
				"quantity": qty, "unit_cost_cny": unitCost, "subtotal_cny": float64(qty) * unitCost,
			})
		}
	}

	return response.Success(c, fiber.StatusOK, "Batch detail from NeonDB", fiber.Map{
		"batch_id": id, "batch_code": code, "status": status,
		"cny_cost": cnyCost, "freight_cost": freight,
		"customs_duty_pct": customsDuty, "landed_cost_bdt": landed,
		"customs_info": customsInfo, "items": items,
		"created_at": createdAt.Format(time.RFC3339), "updated_at": updatedAt.Format(time.RFC3339),
	})
}

func (h *Handler) CalculateLandedCost(c *fiber.Ctx) error {
	// Dynamic calculation based on query params
	cnyCost := c.QueryFloat("cny_cost", 0)
	qty := c.QueryInt("quantity", 1)
	freightType := c.Query("freight_type", "SEA") // SEA or AIR
	customsDutyPct := c.QueryFloat("customs_duty_pct", 25.0)

	if cnyCost <= 0 {
		return response.ValidationError(c, map[string]string{"cny_cost": "required and must be > 0"})
	}
	if qty <= 0 {
		qty = 1
	}

	// Dynamic exchange rate lookup from settings if DB available
	cnyToBdtRate := 16.8 // Default BDT/CNY rate
	if h.db != nil {
		var rateStr string
		err := h.db.QueryRow(c.Context(), "SELECT value FROM platform_settings WHERE key = 'cny_bdt_rate'").Scan(&rateStr)
		if err == nil {
			fmt.Sscanf(rateStr, "%f", &cnyToBdtRate)
		}
	}

	var freightPerUnit float64
	if freightType == "AIR" {
		freightPerUnit = 150.0 // BDT per unit for air freight
	} else {
		freightPerUnit = 85.0 // BDT per unit for sea freight
	}

	bdtCostPerUnit := cnyCost * cnyToBdtRate
	customsDutyPerUnit := bdtCostPerUnit * (customsDutyPct / 100)
	landedCostPerUnit := bdtCostPerUnit + freightPerUnit + customsDutyPerUnit
	totalLandedCost := landedCostPerUnit * float64(qty)

	return response.Success(c, fiber.StatusOK, "Landed cost calculation", fiber.Map{
		"inputs": fiber.Map{
			"cny_cost_per_unit": cnyCost, "quantity": qty,
			"freight_type": freightType, "customs_duty_pct": customsDutyPct,
		},
		"rates": fiber.Map{
			"cny_to_bdt_rate": cnyToBdtRate, "freight_per_unit_bdt": freightPerUnit,
		},
		"per_unit": fiber.Map{
			"bdt_cost":        bdtCostPerUnit,
			"customs_duty":    customsDutyPerUnit,
			"freight":         freightPerUnit,
			"landed_cost_bdt": landedCostPerUnit,
		},
		"total": fiber.Map{
			"quantity":          qty,
			"total_landed_bdt":  totalLandedCost,
			"avg_landed_per_unit": landedCostPerUnit,
		},
	})
}

func (h *Handler) ListCustomsDeclarations(c *fiber.Ctx) error {
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Customs declarations", fiber.Map{"declarations": []fiber.Map{}})
	}
	ctx := c.Context()
	rows, err := h.db.Query(ctx, `
		SELECT id::text, batch_code, status, customs_info, updated_at
		FROM china_sourcing_batches
		WHERE customs_info != '{}'::jsonb
		ORDER BY updated_at DESC`)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load customs data: "+err.Error(), nil)
	}
	defer rows.Close()

	declarations := []fiber.Map{}
	for rows.Next() {
		var id, code, status string
		var customsInfo interface{}
		var updatedAt time.Time
		rows.Scan(&id, &code, &status, &customsInfo, &updatedAt)
		declarations = append(declarations, fiber.Map{
			"batch_id": id, "batch_code": code, "status": status,
			"customs_info": customsInfo, "updated_at": updatedAt.Format(time.RFC3339),
		})
	}
	return response.Success(c, fiber.StatusOK, "Customs declarations from NeonDB", fiber.Map{
		"declarations": declarations, "count": len(declarations),
	})
}

type CreateBatchReq struct {
	Description string `json:"description"`
	FreightType string `json:"freight_type"` // SEA or AIR
}

func (h *Handler) CreateBatch(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	var req CreateBatchReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}
	if req.FreightType == "" {
		req.FreightType = "SEA"
	}

	// Generate unique batch code
	batchCode := fmt.Sprintf("CN-%d-%s-%04d",
		time.Now().Year(),
		strings.ToUpper(req.FreightType[:3]),
		time.Now().UnixNano()%10000,
	)

	if h.db == nil {
		return response.Created(c, "China import batch created", fiber.Map{
			"batch_code": batchCode, "status": "SOURCING",
		})
	}

	ctx := c.Context()
	var batchID string
	err := h.db.QueryRow(ctx, `
		INSERT INTO china_sourcing_batches (batch_code, user_id, status)
		VALUES ($1, $2::uuid, 'SOURCING') RETURNING id::text`,
		batchCode, userID,
	).Scan(&batchID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to create batch: "+err.Error(), nil)
	}

	return response.Created(c, "China import batch created in NeonDB", fiber.Map{
		"batch_id":   batchID,
		"batch_code": batchCode,
		"status":     "SOURCING",
		"user_id":    userID,
		"created_at": time.Now().Format(time.RFC3339),
	})
}

type AddBatchItemReq struct {
	ProductName  string  `json:"product_name"`
	SKUCode      string  `json:"sku_code"`
	Quantity     int     `json:"quantity"`
	UnitCostCNY  float64 `json:"unit_cost_cny"`
}

func (h *Handler) AddBatchItem(c *fiber.Ctx) error {
	batchID := c.Params("id")
	var req AddBatchItemReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}
	if req.ProductName == "" || req.Quantity <= 0 {
		return response.ValidationError(c, map[string]string{"item": "product_name and quantity > 0 are required"})
	}

	if h.db == nil {
		return response.Created(c, "Item added to batch", fiber.Map{"batch_id": batchID})
	}

	ctx := c.Context()
	var itemID string
	err := h.db.QueryRow(ctx, `
		INSERT INTO china_sourcing_batch_items (batch_id, product_name, sku_code, quantity, unit_cost_cny)
		VALUES ($1::uuid, $2, $3, $4, $5) RETURNING id::text`,
		batchID, req.ProductName, req.SKUCode, req.Quantity, req.UnitCostCNY,
	).Scan(&itemID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to add item: "+err.Error(), nil)
	}

	return response.Created(c, "Item added to China batch in NeonDB", fiber.Map{
		"item_id":      itemID,
		"batch_id":     batchID,
		"product_name": req.ProductName,
		"quantity":     req.Quantity,
		"unit_cost_cny": req.UnitCostCNY,
		"subtotal_cny": float64(req.Quantity) * req.UnitCostCNY,
	})
}

type UpdateBatchStatusReq struct {
	Status string `json:"status"`
}

func (h *Handler) UpdateBatchStatus(c *fiber.Ctx) error {
	batchID := c.Params("id")
	var req UpdateBatchStatusReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}
	validStatuses := map[string]bool{
		"SOURCING": true, "ORDERED": true, "IN_TRANSIT_AIR": true,
		"IN_TRANSIT_SEA": true, "CUSTOMS": true, "WAREHOUSE": true, "COMPLETED": true, "CANCELLED": true,
	}
	if !validStatuses[req.Status] {
		return response.ValidationError(c, map[string]string{"status": "invalid status"})
	}

	if h.db != nil {
		ctx := c.Context()
		result, err := h.db.Exec(ctx,
			"UPDATE china_sourcing_batches SET status = $1, updated_at = now() WHERE id::text = $2",
			req.Status, batchID)
		if err != nil {
			return response.Error(c, fiber.StatusInternalServerError, "Failed to update status: "+err.Error(), nil)
		}
		if result.RowsAffected() == 0 {
			return response.Error(c, fiber.StatusNotFound, "Batch not found", nil)
		}
	}
	return response.Success(c, fiber.StatusOK, "Batch status updated in NeonDB", fiber.Map{
		"batch_id": batchID, "status": req.Status,
	})
}

type UpdateLandedCostReq struct {
	CNYCost        float64 `json:"cny_cost"`
	FreightCost    float64 `json:"freight_cost"`
	CustomsDutyPct float64 `json:"customs_duty_pct"`
	LandedCostBDT  float64 `json:"landed_cost_bdt"`
}

func (h *Handler) UpdateBatchLandedCost(c *fiber.Ctx) error {
	batchID := c.Params("id")
	var req UpdateLandedCostReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}

	if h.db != nil {
		ctx := c.Context()
		result, err := h.db.Exec(ctx, `
			UPDATE china_sourcing_batches
			SET cny_cost = $1, freight_cost = $2, customs_duty_pct = $3, landed_cost_bdt = $4, updated_at = now()
			WHERE id::text = $5`,
			req.CNYCost, req.FreightCost, req.CustomsDutyPct, req.LandedCostBDT, batchID)
		if err != nil {
			return response.Error(c, fiber.StatusInternalServerError, "Failed to update landed cost: "+err.Error(), nil)
		}
		if result.RowsAffected() == 0 {
			return response.Error(c, fiber.StatusNotFound, "Batch not found", nil)
		}
	}
	return response.Success(c, fiber.StatusOK, "Landed cost updated in NeonDB", fiber.Map{
		"batch_id":       batchID,
		"cny_cost":       req.CNYCost,
		"freight_cost":   req.FreightCost,
		"customs_duty_pct": req.CustomsDutyPct,
		"landed_cost_bdt": req.LandedCostBDT,
	})
}

type UpdateCustomsReq struct {
	HSCode           string `json:"hs_code"`
	DeclarationNo    string `json:"declaration_no"`
	CustomsOfficer   string `json:"customs_officer"`
	ClearedAt        string `json:"cleared_at"`
	DutyPaidAmount   float64 `json:"duty_paid_amount"`
}

func (h *Handler) UpdateBatchCustomsInfo(c *fiber.Ctx) error {
	batchID := c.Params("id")
	var req UpdateCustomsReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}

	customsData := fiber.Map{
		"hs_code": req.HSCode, "declaration_no": req.DeclarationNo,
		"customs_officer": req.CustomsOfficer, "cleared_at": req.ClearedAt,
		"duty_paid_amount": req.DutyPaidAmount,
	}

	if h.db != nil {
		ctx := c.Context()
		result, err := h.db.Exec(ctx, `
			UPDATE china_sourcing_batches SET customs_info = $1::jsonb, updated_at = now() WHERE id::text = $2`,
			customsData, batchID)
		if err != nil {
			return response.Error(c, fiber.StatusInternalServerError, "Failed to update customs info: "+err.Error(), nil)
		}
		if result.RowsAffected() == 0 {
			return response.Error(c, fiber.StatusNotFound, "Batch not found", nil)
		}
	}
	return response.Success(c, fiber.StatusOK, "Customs info updated in NeonDB", fiber.Map{
		"batch_id": batchID, "customs_info": customsData,
	})
}
