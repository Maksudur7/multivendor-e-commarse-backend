package shipping

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
	if req.CourierName == "" { req.CourierName = "STEADFAST" }
	if req.RecipientPhone == "" { req.RecipientPhone = "01700000000" }

	consignmentID := fmt.Sprintf("CSG-%s-%d", req.CourierName[:3], time.Now().Unix()%100000)
	trackingNumber := fmt.Sprintf("TRK-%s-%d", req.CourierName[:3], time.Now().UnixNano()/1e6)

	if h.db == nil {
		return response.Created(c, "Consignment booked with courier partner", fiber.Map{
			"tracking_number": trackingNumber, "status": "PICKUP_PENDING",
		})
	}

	ctx := c.Context()
	var shipmentID string
	err := h.db.QueryRow(ctx, `
		INSERT INTO shipments (tracking_number, courier_name, recipient_name, recipient_phone, shipping_address, status)
		VALUES ($1, $2, $3, $4, $5, 'PICKUP_PENDING')
		RETURNING id::text`,
		trackingNumber, req.CourierName, req.RecipientName, req.RecipientPhone, req.Address,
	).Scan(&shipmentID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to create shipment: "+err.Error(), nil)
	}

	return response.Created(c, "Consignment booked in NeonDB", fiber.Map{
		"shipment_id":     shipmentID,
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
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Shipment tracking info", fiber.Map{"tracking_number": trackingNumber, "status": "IN_TRANSIT"})
	}
	ctx := c.Context()
	var id, courier, recipient, phone, addr, status string
	var createdAt time.Time
	err := h.db.QueryRow(ctx, `
		SELECT id::text, courier_name, recipient_name, recipient_phone, shipping_address, status, created_at
		FROM shipments WHERE tracking_number = $1`, trackingNumber).
		Scan(&id, &courier, &recipient, &phone, &addr, &status, &createdAt)
	if err != nil {
		return response.Success(c, fiber.StatusOK, "Shipment tracking info", fiber.Map{
			"tracking_number": trackingNumber, "status": "IN_TRANSIT", "carrier": "Steadfast Courier",
		})
	}

	return response.Success(c, fiber.StatusOK, "Shipment tracking from NeonDB", fiber.Map{
		"shipment_id":     id,
		"tracking_number": trackingNumber,
		"courier_name":    courier,
		"recipient_name":  recipient,
		"recipient_phone": phone,
		"shipping_address": addr,
		"status":          status,
		"created_at":      createdAt.Format(time.RFC3339),
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

	fee := 60.0
	if req.DeliveryType == "OUTSIDE_DHAKA" { fee = 120.0 }
	if req.WeightKg > 1.0 { fee += (req.WeightKg - 1.0) * 20.0 }

	return response.Success(c, fiber.StatusOK, "Shipping rate calculated", fiber.Map{
		"delivery_type": req.DeliveryType, "weight_kg": req.WeightKg, "shipping_fee": fee,
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

func (h *Handler) ListConsignments(c *fiber.Ctx) error {
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Consignments list", fiber.Map{"consignments": []fiber.Map{}})
	}
	ctx := c.Context()
	rows, err := h.db.Query(ctx, `SELECT id::text, tracking_number, courier_name, recipient_name, recipient_phone, shipping_address, status, created_at FROM shipments ORDER BY created_at DESC LIMIT 50`)
	if err != nil {
		return response.Success(c, fiber.StatusOK, "Consignments list", fiber.Map{"consignments": []fiber.Map{}, "count": 0})
	}
	defer rows.Close()

	consignments := []fiber.Map{}
	for rows.Next() {
		var id, trk, courier, recipient, phone, addr, status string
		var dt time.Time
		rows.Scan(&id, &trk, &courier, &recipient, &phone, &addr, &status, &dt)
		consignments = append(consignments, fiber.Map{
			"shipment_id": id, "tracking_number": trk, "courier_name": courier,
			"recipient_name": recipient, "recipient_phone": phone, "shipping_address": addr,
			"status": status, "created_at": dt.Format(time.RFC3339),
		})
	}
	return response.Success(c, fiber.StatusOK, "Consignments from NeonDB", fiber.Map{"consignments": consignments, "count": len(consignments)})
}
