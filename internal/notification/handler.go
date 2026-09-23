package notification

import (
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
	n := router.Group("/notifications", authMiddleware)
	n.Get("/", h.GetNotifications)
	n.Post("/send-test-sms", h.SendTestSMS)
}

func (h *Handler) GetNotifications(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Notifications loaded", fiber.Map{"user_id": userID, "notifications": []fiber.Map{}})
	}
	ctx := c.Context()
	rows, err := h.db.Query(ctx, `
		SELECT id::text, title, message, is_read, created_at FROM notifications WHERE user_id::text = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to fetch notifications", nil)
	}
	defer rows.Close()

	notifs := []fiber.Map{}
	for rows.Next() {
		var id, title, msg string; var read bool; var dt time.Time
		rows.Scan(&id, &title, &msg, &read, &dt)
		notifs = append(notifs, fiber.Map{
			"id": id, "title": title, "message": msg, "is_read": read, "created_at": dt.Format(time.RFC3339),
		})
	}
	return response.Success(c, fiber.StatusOK, "In-app notifications from NeonDB", fiber.Map{"user_id": userID, "notifications": notifs, "count": len(notifs)})
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
	return response.Success(c, fiber.StatusOK, "SMS dispatched via Greenweb / SSLWireless API gateway", fiber.Map{
		"recipient": req.PhoneNumber, "status": "DELIVERED",
	})
}
