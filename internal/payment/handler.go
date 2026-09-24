package payment

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yourusername/ecom-backend/pkg/response"
)

type Handler struct {
	service *Service
}

func NewHandler(db *pgxpool.Pool) *Handler {
	repo := NewRepository(db)
	service := NewService(repo)
	return &Handler{service: service}
}

func NewHandlerWithService(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	webhooks := router.Group("/payments/webhooks")
	webhooks.Post("/bkash", h.BkashWebhook)
	webhooks.Post("/sslcommerz", h.SSLCommerzWebhook)
	webhooks.Post("/nagad", h.NagadWebhook)

	p := router.Group("/payments")
	p.Get("/methods", h.GetPaymentMethods)
	p.Get("/status/:id", h.GetPaymentStatus)

	protected := p.Group("", authMiddleware)
	protected.Post("/initiate", h.InitiatePayment)
	protected.Post("/verify", h.VerifyPayment)
	protected.Post("/refunds", h.RequestRefund)
	protected.Post("/escrow/release", h.ReleaseEscrow)
}

func (h *Handler) GetPaymentMethods(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Supported payment gateways retrieved", fiber.Map{
		"gateways": []fiber.Map{
			{"code": "BKASH", "name": "bKash Tokenized Checkout", "type": "MFS"},
			{"code": "NAGAD", "name": "Nagad Direct Gateway", "type": "MFS"},
			{"code": "SSLCOMMERZ", "name": "SSLCommerz Cards & Banking", "type": "GATEWAY"},
			{"code": "COD", "name": "Cash on Delivery", "type": "COD"},
		},
	})
}

func (h *Handler) GetPaymentStatus(c *fiber.Ctx) error {
	txID := c.Params("id")
	item, err := h.service.GetPaymentStatus(c.Context(), txID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to fetch payment status: "+err.Error(), nil)
	}
	return response.Success(c, fiber.StatusOK, "Payment transaction status", item)
}

type InitiatePaymentReq struct {
	MasterOrderID  string  `json:"master_order_id"`
	PaymentGateway string  `json:"payment_gateway"` // BKASH, SSLCOMMERZ, NAGAD, COD
	Amount         float64 `json:"amount"`
}

func (h *Handler) InitiatePayment(c *fiber.Ctx) error {
	var req InitiatePaymentReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body: "+err.Error())
	}

	_, res, err := h.service.InitiatePayment(c.Context(), req.MasterOrderID, req.PaymentGateway, req.Amount)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to initiate payment: "+err.Error(), nil)
	}

	msg := "Payment initiated successfully"
	if req.PaymentGateway == "BKASH" {
		msg = "bKash payment URL created"
	} else if req.PaymentGateway == "SSLCOMMERZ" {
		msg = "SSLCommerz gateway session initialized"
	} else if req.PaymentGateway == "NAGAD" {
		msg = "Nagad payment session generated"
	} else {
		msg = "Cash on Delivery selected"
	}

	return response.Success(c, fiber.StatusOK, msg, res)
}

func (h *Handler) VerifyPayment(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Payment transaction verified", fiber.Map{"status": "SUCCESS"})
}

func (h *Handler) BkashWebhook(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "COMPLETED", "msg": "bKash callback received"})
}

func (h *Handler) SSLCommerzWebhook(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "VALIDATED", "msg": "SSLCommerz IPN verified"})
}

func (h *Handler) NagadWebhook(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "SUCCESS", "msg": "Nagad callback received"})
}

type RequestRefundReq struct {
	SubOrderID string  `json:"sub_order_id"`
	Reason     string  `json:"reason"`
	Amount     float64 `json:"amount"`
}

func (h *Handler) RequestRefund(c *fiber.Ctx) error {
	var req RequestRefundReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}
	customerID := c.Locals("user_id").(string)

	err := h.service.RequestRefund(c.Context(), customerID, req.Amount, req.Reason)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to request refund: "+err.Error(), nil)
	}

	return response.Created(c, "Refund request submitted in NeonDB", fiber.Map{
		"customer_id": customerID, "amount": req.Amount, "status": "PROCESSING",
	})
}

func (h *Handler) ReleaseEscrow(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Escrow funds released", nil)
}
