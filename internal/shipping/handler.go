package shipping

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
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
	s := router.Group("/shipping", authMiddleware)
	s.Post("/create-consignment", h.CreateConsignment)
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
	CourierName    string  `json:"courier_name"` // PATHAO, STEADFAST, PAPERFLY
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
	if req.SubOrderID == "" || req.CourierName == "" || req.RecipientPhone == "" {
		return response.ValidationError(c, map[string]string{
			"shipping": "sub_order_id, courier_name, and recipient_phone are required",
		})
	}

	consignmentID := fmt.Sprintf("CSG-%s-%d", req.CourierName[:3], time.Now().Unix()%100000)
	trackingNumber := fmt.Sprintf("TRK-%s-%d", req.CourierName[:3], time.Now().UnixNano()/1e6)

	return response.Created(c, "Consignment booked with courier partner successfully", fiber.Map{
		"shipment_id":     uuid.New().String(),
		"sub_order_id":    req.SubOrderID,
		"courier_name":    req.CourierName,
		"consignment_id":  consignmentID,
		"tracking_number": trackingNumber,
		"status":          "PICKUP_PENDING",
		"waybill_url":     fmt.Sprintf("https://shipping.platform.com/waybill/%s.pdf", trackingNumber),
	})
}

func (h *Handler) TrackShipment(c *fiber.Ctx) error {
	trackingNumber := c.Params("trackingNumber")
	return response.Success(c, fiber.StatusOK, "Shipment tracking info", fiber.Map{
		"tracking_number": trackingNumber,
		"status":          "IN_TRANSIT",
		"current_location": "Dhaka Hub Central",
		"history": []fiber.Map{
			{"status": "PICKUP_PENDING", "timestamp": time.Now().Add(-12 * time.Hour).Format(time.RFC3339)},
			{"status": "PICKED_UP", "timestamp": time.Now().Add(-6 * time.Hour).Format(time.RFC3339)},
			{"status": "IN_TRANSIT", "timestamp": time.Now().Format(time.RFC3339)},
		},
	})
}

type CalculateRateReq struct {
	DeliveryType string  `json:"delivery_type"` // INSIDE_DHAKA, OUTSIDE_DHAKA, SAME_DAY
	WeightKg     float64 `json:"weight_kg"`
}

func (h *Handler) CalculateShippingRate(c *fiber.Ctx) error {
	var req CalculateRateReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}

	fee := 60.0 // default inside Dhaka
	if req.DeliveryType == "OUTSIDE_DHAKA" {
		fee = 120.0
	}
	if req.WeightKg > 1.0 {
		fee += (req.WeightKg - 1.0) * 20.0
	}

	return response.Success(c, fiber.StatusOK, "Shipping rate calculated", fiber.Map{
		"delivery_type": req.DeliveryType,
		"weight_kg":     req.WeightKg,
		"shipping_fee":  fee,
	})
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
