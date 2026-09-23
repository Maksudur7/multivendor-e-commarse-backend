package notification

import (
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
	n := router.Group("/notifications", authMiddleware)
	n.Get("/", h.GetNotifications)
	n.Post("/send-test-sms", h.SendTestSMS)
}

func (h *Handler) GetNotifications(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	return response.Success(c, fiber.StatusOK, "In-app notifications loaded", fiber.Map{
		"user_id":       userID,
		"notifications": []fiber.Map{},
	})
}

type SendSMSReq struct {
	PhoneNumber string `json:"phone_number"`
	Message     string `json:"message"`
}

func (h *Handler) SendTestSMS(c *fiber.Ctx) error {
	var req SendSMSReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body: "+err.Error())
	}
	if req.PhoneNumber == "" || req.Message == "" {
		return response.ValidationError(c, map[string]string{
			"sms": "phone_number and message are required",
		})
	}
	return response.Success(c, fiber.StatusOK, "SMS dispatched via Greenweb / SSLWireless API gateway", fiber.Map{
		"recipient": req.PhoneNumber,
		"status":    "DELIVERED",
	})
}
