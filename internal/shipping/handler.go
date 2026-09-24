package shipping

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
	// Public tracking endpoint
	router.Get("/shipping/track/:trackingNumber", h.TrackShipment)

	s := router.Group("/shipping", authMiddleware)
	s.Post("/create-consignment", h.CreateConsignment)
	s.Get("/consignments", h.ListConsignments)
	s.Get("/track/:trackingNumber", h.TrackShipment)
	s.Post("/calculate-rate", h.CalculateShippingRate)

	// Courier Webhook status update callbacks
	webhooks := router.Group("/shipping/webhooks")
	webhooks.Post("/pathao", h.PathaoWebhook)
	webhooks.Post("/steadfast", h.SteadfastWebhook)
	webhooks.Post("/paperfly", h.PaperflyWebhook)
}

type CreateConsignmentReq struct {
	SubOrderID     string  `json:"sub_order_id"`
	CourierName    string  `json:"courier_name"`
	RecipientName  string  `json:"recipient_name"`
	RecipientPhone string  `json:"recipient_phone"`
	Address        string  `json:"address"`
	CityID         int     `json:"city_id"`
	ZoneID         int     `json:"zone_id"`
	ItemWeightKg   float64 `json:"item_weight_kg"`
	CODAmount      float64 `json:"cod_amount"`
}

func (h *Handler) CreateConsignment(c *fiber.Ctx) error {
	var req CreateConsignmentReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}

	result, err := h.service.CreateConsignment(c.Context(), req.SubOrderID, req.CourierName, req.RecipientName, req.RecipientPhone, req.Address)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to create shipment: "+err.Error(), nil)
	}

	return response.Created(c, "Consignment booked with courier partner", result)
}

func (h *Handler) TrackShipment(c *fiber.Ctx) error {
	trackingNumber := c.Params("trackingNumber")
	shipment, err := h.service.TrackShipment(c.Context(), trackingNumber)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error(), nil)
	}
	return response.Success(c, fiber.StatusOK, "Shipment tracking info", shipment)
}

type CalculateRateReq struct {
	DeliveryType string  `json:"delivery_type"`
	WeightKg     float64 `json:"weight_kg"`
}

func (h *Handler) CalculateShippingRate(c *fiber.Ctx) error {
	var req CalculateRateReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}
	result := h.service.CalculateShippingRate(req.DeliveryType, req.WeightKg)
	return response.Success(c, fiber.StatusOK, "Shipping rate calculated", result)
}

func (h *Handler) PathaoWebhook(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "Pathao webhook received"})
}

func (h *Handler) SteadfastWebhook(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "Steadfast webhook received"})
}

func (h *Handler) PaperflyWebhook(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "Paperfly webhook received"})
}

func (h *Handler) ListConsignments(c *fiber.Ctx) error {
	consignments, err := h.service.ListConsignments(c.Context())
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to list consignments", nil)
	}
	return response.Success(c, fiber.StatusOK, "Consignments list", fiber.Map{"consignments": consignments, "count": len(consignments)})
}
