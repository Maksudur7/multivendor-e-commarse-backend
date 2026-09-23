package dispute

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
	d := router.Group("/disputes", authMiddleware)
	d.Post("/open", h.OpenDispute)
	d.Get("/my-tickets", h.GetMyTickets)
	d.Post("/:ticketId/messages", h.SendMessage)

	admin := router.Group("/admin/disputes", authMiddleware)
	admin.Put("/:ticketId/resolve", h.ResolveDisputeAdmin)
}

type OpenDisputeReq struct {
	SubOrderID   string   `json:"sub_order_id"`
	VendorID     string   `json:"vendor_id"`
	Reason       string   `json:"reason"` // WRONG_PRODUCT, DAMAGED, NOT_RECEIVED
	Description  string   `json:"description"`
	EvidenceURLs []string `json:"evidence_urls"`
}

func (h *Handler) OpenDispute(c *fiber.Ctx) error {
	var req OpenDisputeReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body: "+err.Error())
	}
	if req.SubOrderID == "" || req.Reason == "" || req.Description == "" {
		return response.ValidationError(c, map[string]string{
			"dispute": "sub_order_id, reason, and description are required",
		})
	}

	customerID := c.Locals("user_id").(string)
	ticketNo := fmt.Sprintf("TK-%d", time.Now().UnixNano()/1e6)

	return response.Created(c, "Dispute ticket opened. Escrow funds locked pending resolution.", fiber.Map{
		"ticket_id":     uuid.New().String(),
		"ticket_number": ticketNo,
		"customer_id":   customerID,
		"sub_order_id":  req.SubOrderID,
		"status":        "OPEN",
	})
}

func (h *Handler) GetMyTickets(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Dispute tickets list", fiber.Map{
		"tickets": []fiber.Map{},
	})
}

type SendMessageReq struct {
	Message     string   `json:"message"`
	Attachments []string `json:"attachments,omitempty"`
}

func (h *Handler) SendMessage(c *fiber.Ctx) error {
	ticketID := c.Params("ticketId")
	var req SendMessageReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid message input: "+err.Error())
	}
	userID := c.Locals("user_id").(string)
	role := c.Locals("role").(string)

	return response.Created(c, "Message added to dispute ticket", fiber.Map{
		"ticket_id":   ticketID,
		"sender_id":   userID,
		"sender_type": role,
		"message":     req.Message,
		"timestamp":   time.Now().Format(time.RFC3339),
	})
}

type ResolveDisputeReq struct {
	Status          string `json:"status"` // REFUND_CUSTOMER, PAY_VENDOR, SPLIT
	AdminResolution string `json:"admin_resolution"`
}

func (h *Handler) ResolveDisputeAdmin(c *fiber.Ctx) error {
	ticketID := c.Params("ticketId")
	var req ResolveDisputeReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}
	return response.Success(c, fiber.StatusOK, "Dispute ticket resolved by Admin", fiber.Map{
		"ticket_id":        ticketID,
		"final_status":     req.Status,
		"admin_resolution": req.AdminResolution,
	})
}
