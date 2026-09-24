package chinasourcing

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yourusername/ecom-backend/pkg/response"
)

type Handler struct {
	repo    *Repository
	service *Service
}

func NewHandler(db *pgxpool.Pool) *Handler {
	repo := NewRepository(db)
	service := NewService(repo)
	return &Handler{repo: repo, service: service}
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
	batches, err := h.service.ListBatches(c.Context(), userID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load batches: "+err.Error(), nil)
	}
	return response.Success(c, fiber.StatusOK, "China import batches loaded", fiber.Map{
		"batches": batches, "count": len(batches),
	})
}

func (h *Handler) GetBatchDetail(c *fiber.Ctx) error {
	batchID := c.Params("id")
	detail, err := h.service.GetBatchDetail(c.Context(), batchID)
	if err != nil {
		return response.Error(c, fiber.StatusNotFound, "Batch not found", nil)
	}
	return response.Success(c, fiber.StatusOK, "Batch detail loaded", detail)
}

func (h *Handler) CalculateLandedCost(c *fiber.Ctx) error {
	cnyCost := c.QueryFloat("cny_cost", 0)
	qty := c.QueryInt("quantity", 1)
	freightType := c.Query("freight_type", "SEA")
	customsDutyPct := c.QueryFloat("customs_duty_pct", 25.0)

	calc, err := h.service.CalculateLandedCost(cnyCost, qty, freightType, customsDutyPct)
	if err != nil {
		return response.ValidationError(c, map[string]string{"cny_cost": err.Error()})
	}
	return response.Success(c, fiber.StatusOK, "Landed cost calculation", calc)
}

func (h *Handler) ListCustomsDeclarations(c *fiber.Ctx) error {
	declarations, err := h.service.ListCustomsDeclarations(c.Context())
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load customs data: "+err.Error(), nil)
	}
	return response.Success(c, fiber.StatusOK, "Customs declarations loaded", fiber.Map{
		"declarations": declarations, "count": len(declarations),
	})
}

type CreateBatchReq struct {
	Description string `json:"description"`
	FreightType string `json:"freight_type"`
}

func (h *Handler) CreateBatch(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	var req CreateBatchReq
	_ = c.BodyParser(&req)

	batchID, batchCode, err := h.service.CreateBatch(c.Context(), userID, req.FreightType)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to create batch: "+err.Error(), nil)
	}

	return response.Created(c, "China import batch created", fiber.Map{
		"batch_id":   batchID,
		"batch_code": batchCode,
		"status":     "SOURCING",
		"user_id":    userID,
		"created_at": time.Now().Format(time.RFC3339),
	})
}

type AddBatchItemReq struct {
	ProductName string  `json:"product_name"`
	SKUCode     string  `json:"sku_code"`
	Quantity    int     `json:"quantity"`
	UnitCostCNY float64 `json:"unit_cost_cny"`
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

	itemID, err := h.service.AddBatchItem(c.Context(), batchID, req.ProductName, req.SKUCode, req.Quantity, req.UnitCostCNY)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to add item: "+err.Error(), nil)
	}

	return response.Created(c, "Item added to China batch", fiber.Map{
		"item_id":       itemID,
		"batch_id":      batchID,
		"product_name":  req.ProductName,
		"quantity":      req.Quantity,
		"unit_cost_cny": req.UnitCostCNY,
		"subtotal_cny":  float64(req.Quantity) * req.UnitCostCNY,
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

	if err := h.service.UpdateBatchStatus(c.Context(), batchID, req.Status); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, err.Error(), nil)
	}
	return response.Success(c, fiber.StatusOK, "Batch status updated", fiber.Map{
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

	if err := h.service.UpdateBatchLandedCost(c.Context(), batchID, req.CNYCost, req.FreightCost, req.CustomsDutyPct, req.LandedCostBDT); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, err.Error(), nil)
	}
	return response.Success(c, fiber.StatusOK, "Landed cost updated", fiber.Map{
		"batch_id":         batchID,
		"cny_cost":         req.CNYCost,
		"freight_cost":     req.FreightCost,
		"customs_duty_pct": req.CustomsDutyPct,
		"landed_cost_bdt":  req.LandedCostBDT,
	})
}

type UpdateCustomsReq struct {
	HSCode         string  `json:"hs_code"`
	DeclarationNo  string  `json:"declaration_no"`
	CustomsOfficer string  `json:"customs_officer"`
	ClearedAt      string  `json:"cleared_at"`
	DutyPaidAmount float64 `json:"duty_paid_amount"`
}

func (h *Handler) UpdateBatchCustomsInfo(c *fiber.Ctx) error {
	batchID := c.Params("id")
	var req UpdateCustomsReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}

	customsData := map[string]interface{}{
		"hs_code": req.HSCode, "declaration_no": req.DeclarationNo,
		"customs_officer": req.CustomsOfficer, "cleared_at": req.ClearedAt,
		"duty_paid_amount": req.DutyPaidAmount,
	}

	if err := h.service.UpdateBatchCustomsInfo(c.Context(), batchID, customsData); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, err.Error(), nil)
	}
	return response.Success(c, fiber.StatusOK, "Customs info updated", fiber.Map{
		"batch_id": batchID, "customs_info": customsData,
	})
}
