package analytics

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
	a := router.Group("/analytics", authMiddleware)

	// 6 Analytics Endpoints
	a.Get("/overview", h.GetOverview)
	a.Get("/sales", h.GetSalesAnalytics)
	a.Get("/customers", h.GetCustomerAnalytics)
	a.Get("/products", h.GetProductAnalytics)
	a.Get("/traffic", h.GetTrafficAnalytics)
	a.Post("/custom-report", h.GenerateCustomReport)
}

func (h *Handler) GetOverview(c *fiber.Ctx) error {
	overview, err := h.service.GetOverview(c.Context())
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load overview", nil)
	}
	return response.Success(c, fiber.StatusOK, "Analytics overview loaded", overview)
}

func (h *Handler) GetSalesAnalytics(c *fiber.Ctx) error {
	days := c.QueryInt("days", 30)
	sales, err := h.service.GetSalesAnalytics(c.Context(), days)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load sales analytics", nil)
	}
	return response.Success(c, fiber.StatusOK, "Sales analytics loaded", sales)
}

func (h *Handler) GetCustomerAnalytics(c *fiber.Ctx) error {
	customers, err := h.service.GetCustomerAnalytics(c.Context())
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load customer analytics", nil)
	}
	return response.Success(c, fiber.StatusOK, "Customer analytics loaded", customers)
}

func (h *Handler) GetProductAnalytics(c *fiber.Ctx) error {
	products, err := h.service.GetProductAnalytics(c.Context())
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load product analytics", nil)
	}
	return response.Success(c, fiber.StatusOK, "Product analytics loaded", products)
}

func (h *Handler) GetTrafficAnalytics(c *fiber.Ctx) error {
	traffic, err := h.service.GetTrafficAnalytics(c.Context())
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load traffic analytics", nil)
	}
	return response.Success(c, fiber.StatusOK, "Traffic analytics loaded", traffic)
}

type CustomReportReq struct {
	ReportType string                 `json:"report_type"`
	Parameters map[string]interface{} `json:"parameters"`
}

func (h *Handler) GenerateCustomReport(c *fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	var req CustomReportReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid payload: "+err.Error())
	}
	if req.ReportType == "" {
		return response.ValidationError(c, map[string]string{"report_type": "required"})
	}

	jobID, err := h.service.QueueCustomReport(c.Context(), req.ReportType, req.Parameters, userID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to queue report: "+err.Error(), nil)
	}

	return response.Created(c, "Custom report job queued", fiber.Map{
		"job_id": jobID, "report_type": req.ReportType, "status": "QUEUED",
		"requested_by": userID, "created_at": time.Now().Format(time.RFC3339),
	})
}
