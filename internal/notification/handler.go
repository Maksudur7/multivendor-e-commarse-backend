package notification

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
	n := router.Group("/notifications", authMiddleware)
	n.Get("/", h.GetNotifications)
	n.Post("/send-test-sms", h.SendTestSMS)
}

func (h *Handler) GetNotifications(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	notifs, err := h.service.GetNotifications(c.Context(), userID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to fetch notifications", nil)
	}
	return response.Success(c, fiber.StatusOK, "In-app notifications loaded", fiber.Map{
		"user_id": userID, "notifications": notifs, "count": len(notifs),
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
		return response.ValidationError(c, map[string]string{"sms": "phone_number and message are required"})
	}

	status, err := h.service.SendSMS(req.PhoneNumber, req.Message)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	return response.Success(c, fiber.StatusOK, "SMS dispatched via Greenweb / SSLWireless API gateway", fiber.Map{
		"recipient": req.PhoneNumber, "status": status,
	})
}
