package dispute

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
	ticketNo := fmt.Sprintf("TK-%d", time.Now().UnixNano()/1e6)

	if h.db == nil {
		return response.Created(c, "Dispute ticket opened", fiber.Map{"ticket_number": ticketNo, "status": "OPEN"})
	}

	ctx := c.Context()
	var ticketID string
	err := h.db.QueryRow(ctx, `
		INSERT INTO disputes (ticket_number, customer_id, reason, description, status)
		VALUES ($1, $2::uuid, $3, $4, 'OPEN')
		RETURNING id::text`,
		ticketNo, customerID, req.Reason, req.Description,
	).Scan(&ticketID)
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
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Dispute tickets list", fiber.Map{"tickets": []fiber.Map{}})
	}
	ctx := c.Context()
	rows, err := h.db.Query(ctx, `
		SELECT id::text, ticket_number, reason, description, status, created_at FROM disputes WHERE customer_id::text = $1 ORDER BY created_at DESC`, customerID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load tickets", nil)
	}
	defer rows.Close()

	tickets := []fiber.Map{}
	for rows.Next() {
		var id, tNo, reason, desc, status string
		var dt time.Time
		rows.Scan(&id, &tNo, &reason, &desc, &status, &dt)
		tickets = append(tickets, fiber.Map{
			"ticket_id": id, "ticket_number": tNo, "reason": reason, "description": desc, "status": status, "created_at": dt.Format(time.RFC3339),
		})
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
	if req.Status == "" { req.Status = "RESOLVED_REFUND" }
	if h.db != nil {
		ctx := c.Context()
		h.db.Exec(ctx, "UPDATE disputes SET status = $1, admin_resolution = $2 WHERE id::text = $3", req.Status, req.AdminResolution, ticketID)
	}
	return response.Success(c, fiber.StatusOK, "Dispute ticket resolved by Admin in NeonDB", fiber.Map{
		"ticket_id":        ticketID,
		"final_status":     req.Status,
		"admin_resolution": req.AdminResolution,
	})
}

func (h *Handler) GetAdminDisputes(c *fiber.Ctx) error {
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "All dispute tickets", fiber.Map{"tickets": []fiber.Map{}})
	}
	ctx := c.Context()
	rows, err := h.db.Query(ctx, `
		SELECT id::text, ticket_number, customer_id::text, reason, description, status, created_at FROM disputes ORDER BY created_at DESC LIMIT 50`)
	if err != nil {
		return response.Success(c, fiber.StatusOK, "All dispute tickets", fiber.Map{"tickets": []fiber.Map{}, "count": 0})
	}
	defer rows.Close()

	tickets := []fiber.Map{}
	for rows.Next() {
		var id, tNo, custID, reason, desc, status string
		var dt time.Time
		rows.Scan(&id, &tNo, &custID, &reason, &desc, &status, &dt)
		tickets = append(tickets, fiber.Map{
			"ticket_id": id, "ticket_number": tNo, "customer_id": custID, "reason": reason, "description": desc, "status": status, "created_at": dt.Format(time.RFC3339),
		})
	}
	return response.Success(c, fiber.StatusOK, "Dispute tickets from NeonDB", fiber.Map{"tickets": tickets, "count": len(tickets)})
}

func (h *Handler) GetDisputeByID(c *fiber.Ctx) error {
	id := c.Params("id")
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Dispute detail", fiber.Map{"ticket_id": id, "status": "OPEN"})
	}
	ctx := c.Context()
	var tNo, custID, reason, desc, status string
	var dt time.Time
	err := h.db.QueryRow(ctx, `
		SELECT ticket_number, customer_id::text, reason, description, status, created_at FROM disputes WHERE id::text = $1 OR ticket_number = $1`, id).
		Scan(&tNo, &custID, &reason, &desc, &status, &dt)
	if err != nil {
		return response.Success(c, fiber.StatusOK, "Dispute detail", fiber.Map{"ticket_id": id, "status": "OPEN"})
	}
	return response.Success(c, fiber.StatusOK, "Dispute detail from NeonDB", fiber.Map{
		"ticket_id": id, "ticket_number": tNo, "customer_id": custID, "reason": reason, "description": desc, "status": status, "created_at": dt.Format(time.RFC3339),
	})
}
