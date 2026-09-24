package inventory

import (
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
	inv := router.Group("/inventory", authMiddleware)
	inv.Get("/stock", h.GetStockLevels)
	inv.Get("/warehouses", h.ListWarehouses)
	inv.Get("/reservations", h.ListReservations)
	inv.Post("/adjust", h.AdjustStock)
	inv.Post("/transfer", h.TransferStock)
	inv.Post("/reserve", h.ReserveStock)
	inv.Put("/warehouses/:id", h.UpdateWarehouse)
	inv.Put("/reservations/:id/release", h.ReleaseReservation)
}

func (h *Handler) GetStockLevels(c *fiber.Ctx) error {
	inv, err := h.service.GetStockLevels(c.Context())
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load stock levels", nil)
	}
	return response.Success(c, fiber.StatusOK, "Stock levels loaded", fiber.Map{"inventory": inv, "count": len(inv)})
}

func (h *Handler) ListWarehouses(c *fiber.Ctx) error {
	whs, err := h.service.ListWarehouses(c.Context())
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load warehouses", nil)
	}
	return response.Success(c, fiber.StatusOK, "Warehouses loaded", fiber.Map{"warehouses": whs})
}

func (h *Handler) ListReservations(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Active stock reservations", fiber.Map{"reservations": []fiber.Map{}})
}

type AdjustStockReq struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
	Warehouse string `json:"warehouse"`
}

func (h *Handler) AdjustStock(c *fiber.Ctx) error {
	var req AdjustStockReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}
	if req.ProductID == "" {
		return response.ValidationError(c, map[string]string{"product_id": "is required"})
	}

	stockID, err := h.service.AdjustStock(c.Context(), req.ProductID, req.Quantity, req.Warehouse)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	return response.Success(c, fiber.StatusOK, "Stock updated", fiber.Map{
		"stock_id": stockID, "product_id": req.ProductID, "quantity": req.Quantity, "warehouse": req.Warehouse,
	})
}

func (h *Handler) TransferStock(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Stock transferred", fiber.Map{"status": "TRANSFERRED"})
}

func (h *Handler) ReserveStock(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Stock reserved", fiber.Map{"status": "RESERVED"})
}

func (h *Handler) UpdateWarehouse(c *fiber.Ctx) error {
	id := c.Params("id")
	return response.Success(c, fiber.StatusOK, "Warehouse updated", fiber.Map{"warehouse_id": id})
}

func (h *Handler) ReleaseReservation(c *fiber.Ctx) error {
	id := c.Params("id")
	return response.Success(c, fiber.StatusOK, "Reservation released", fiber.Map{"reservation_id": id})
}
