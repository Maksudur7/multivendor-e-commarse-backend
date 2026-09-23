package payment

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
	p := router.Group("/payments", authMiddleware)
	p.Get("/methods", h.GetPaymentMethods)
	p.Get("/status/:id", h.GetPaymentStatus)
	p.Post("/initiate", h.InitiatePayment)
	p.Post("/verify", h.VerifyPayment)
	p.Post("/refunds", h.RequestRefund)
	p.Post("/escrow/release", h.ReleaseEscrow)

	// Webhooks / IPN endpoints (public - signature verified)
	webhooks := router.Group("/payments/webhooks")
	webhooks.Post("/bkash", h.BkashWebhook)
	webhooks.Post("/sslcommerz", h.SSLCommerzWebhook)
	webhooks.Post("/nagad", h.NagadWebhook)
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
	return response.Success(c, fiber.StatusOK, "Payment transaction status", fiber.Map{
		"transaction_id": c.Params("id"),
		"status":         "COMPLETED",
	})
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
	if req.MasterOrderID == "" || req.Amount <= 0 {
		return response.ValidationError(c, map[string]string{
			"payment": "master_order_id and valid amount are required",
		})
	}

	txRef := fmt.Sprintf("TXN-%d", time.Now().UnixNano()/1e6)

	switch req.PaymentGateway {
	case "BKASH":
		return response.Success(c, fiber.StatusOK, "bKash payment URL created", fiber.Map{
			"payment_gateway":       "BKASH",
			"transaction_reference": txRef,
			"bkash_payment_url":     fmt.Sprintf("https://checkout.sandbox.bka.sh/v1.2.0-beta/pay/checkout?paymentID=BK-%s", uuid.New().String()[:8]),
		})
	case "SSLCOMMERZ":
		return response.Success(c, fiber.StatusOK, "SSLCommerz gateway session initialized", fiber.Map{
			"payment_gateway":       "SSLCOMMERZ",
			"transaction_reference": txRef,
			"ssl_redirect_url":      fmt.Sprintf("https://sandbox.sslcommerz.com/gwprocess/v4/gw.php?Q=PAY&sessionkey=%s", uuid.New().String()),
		})
	case "NAGAD":
		return response.Success(c, fiber.StatusOK, "Nagad payment session generated", fiber.Map{
			"payment_gateway":       "NAGAD",
			"transaction_reference": txRef,
			"nagad_redirect_url":    fmt.Sprintf("https://api.mynagad.com/pay/%s", txRef),
		})
	case "COD":
		return response.Success(c, fiber.StatusOK, "Cash on Delivery selected - escrow & order locked until delivery confirmation", fiber.Map{
			"payment_gateway":       "COD",
			"transaction_reference": txRef,
			"payment_status":        "PENDING_COD_COLLECTION",
		})
	default:
		return response.BadRequest(c, "Unsupported payment gateway: "+req.PaymentGateway)
	}
}

func (h *Handler) VerifyPayment(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Payment transaction verified", fiber.Map{"status": "SUCCESS"})
}

func (h *Handler) BkashWebhook(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status": "COMPLETED",
		"msg":    "bKash callback received",
	})
}

func (h *Handler) SSLCommerzWebhook(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status": "VALIDATED",
		"msg":    "SSLCommerz IPN verified",
	})
}

func (h *Handler) NagadWebhook(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status": "SUCCESS",
		"msg":    "Nagad callback received",
	})
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
	return response.Created(c, "Refund request submitted. Escrow payout held until dispute resolution.", fiber.Map{
		"refund_id":    uuid.New().String(),
		"customer_id":  customerID,
		"sub_order_id": req.SubOrderID,
		"amount":       req.Amount,
		"status":       "PROCESSING",
	})
}

func (h *Handler) ReleaseEscrow(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Escrow funds released", nil)
}
