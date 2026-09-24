package dispute

import (
	"time"

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
	d := router.Group("/disputes", authMiddleware)
	d.Post("/open", h.OpenDispute)
	d.Get("/my", h.GetMyTickets)
	d.Get("/my-tickets", h.GetMyTickets)
	d.Get("/admin", h.GetAdminDisputes)
	d.Get("/:id", h.GetDisputeByID)
	d.Put("/:id/resolve", h.ResolveDisputeAdmin)
	d.Post("/:ticketId/messages", h.SendMessage)

	admin := router.Group("/admin/disputes", authMiddleware)
	admin.Get("", h.GetAdminDisputes)
	admin.Put("/:ticketId/resolve", h.ResolveDisputeAdmin)
}

type OpenDisputeReq struct {
	SubOrderID   string   `json:"sub_order_id"`
	VendorID     string   `json:"vendor_id"`
	Reason       string   `json:"reason"`
	Description  string   `json:"description"`
	EvidenceURLs []string `json:"evidence_urls"`
}

func (h *Handler) OpenDispute(c *fiber.Ctx) error {
	var req OpenDisputeReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body: "+err.Error())
	}
	if req.Reason == "" || req.Description == "" {
		return response.ValidationError(c, map[string]string{
			"dispute": "reason and description are required",
		})
	}

	customerID := c.Locals("user_id").(string)
	ticketID, ticketNo, err := h.service.OpenDispute(c.Context(), customerID, req.Reason, req.Description)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to open dispute ticket: "+err.Error(), nil)
	}

	return response.Created(c, "Dispute ticket opened in NeonDB. Escrow funds locked pending resolution.", fiber.Map{
		"ticket_id":     ticketID,
		"ticket_number": ticketNo,
		"customer_id":   customerID,
		"status":        "OPEN",
	})
}

func (h *Handler) GetMyTickets(c *fiber.Ctx) error {
	customerID := c.Locals("user_id").(string)
	tickets, err := h.service.GetMyTickets(c.Context(), customerID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load tickets: "+err.Error(), nil)
	}
	return response.Success(c, fiber.StatusOK, "Dispute tickets from NeonDB", fiber.Map{"tickets": tickets, "count": len(tickets)})
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

	return response.Created(c, "Message added to dispute ticket in NeonDB", fiber.Map{
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
	if req.Status == "" {
		req.Status = "RESOLVED_REFUND"
	}
	err := h.service.ResolveDispute(c.Context(), ticketID, req.Status, req.AdminResolution)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to resolve dispute: "+err.Error(), nil)
	}
	return response.Success(c, fiber.StatusOK, "Dispute ticket resolved by Admin in NeonDB", fiber.Map{
		"ticket_id":        ticketID,
		"final_status":     req.Status,
		"admin_resolution": req.AdminResolution,
	})
}

func (h *Handler) GetAdminDisputes(c *fiber.Ctx) error {
	tickets, err := h.service.GetAllDisputes(c.Context())
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load dispute tickets: "+err.Error(), nil)
	}
	return response.Success(c, fiber.StatusOK, "Dispute tickets from NeonDB", fiber.Map{"tickets": tickets, "count": len(tickets)})
}

func (h *Handler) GetDisputeByID(c *fiber.Ctx) error {
	id := c.Params("id")
	item, err := h.service.GetDisputeByID(c.Context(), id)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to fetch dispute detail: "+err.Error(), nil)
	}
	return response.Success(c, fiber.StatusOK, "Dispute detail from NeonDB", item)
}
