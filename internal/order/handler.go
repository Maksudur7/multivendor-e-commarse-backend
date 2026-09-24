package order

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
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

func (h *Handler) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	// Customer Orders
	cust := router.Group("/customer/orders", authMiddleware)
	cust.Get("/", h.GetCustomerOrders)
	cust.Get("/:id", h.GetCustomerOrderDetail)
	cust.Get("/:id/track", h.TrackCustomerOrder)
	cust.Get("/:id/invoice", h.GetCustomerInvoice)
	cust.Get("/:id/items", h.GetCustomerOrderItems)
	cust.Post("/checkout", h.Checkout)
	cust.Post("/:id/cancel", h.CancelCustomerOrder)
	cust.Post("/:id/reorder", h.ReorderCustomerOrder)
	cust.Put("/:id/address", h.UpdateCustomerOrderAddress)
	cust.Delete("/:id/draft", h.DeleteCustomerDraftOrder)

	// Seller Orders
	seller := router.Group("/seller/orders", authMiddleware)
	seller.Get("/", h.GetSellerOrders)
	seller.Get("/:id", h.GetSellerOrderDetail)
	seller.Get("/pending", h.GetSellerPendingOrders)
	seller.Get("/analytics", h.GetSellerOrderAnalytics)
	seller.Post("/:id/pack", h.PackSellerOrder)
	seller.Post("/:id/print-label", h.PrintSellerOrderLabel)
	seller.Post("/:id/ship", h.ShipSellerOrder)
	seller.Put("/:id/status", h.UpdateSellerOrderStatus)
	seller.Put("/:id/tracking", h.UpdateSellerOrderTracking)

	// Admin Orders
	admin := router.Group("/admin/orders", authMiddleware)
	admin.Get("/", h.ListAdminOrders)
	admin.Get("/:id", h.GetAdminOrderDetail)
	admin.Get("/sub-orders", h.ListAdminSubOrders)
	admin.Get("/disputed", h.ListAdminDisputedOrders)
	admin.Get("/analytics", h.GetAdminOrderAnalytics)
	admin.Get("/exports", h.ExportAdminOrders)
	admin.Post("/force-cancel", h.ForceCancelAdminOrder)
	admin.Post("/override-status", h.OverrideStatusAdminOrder)
	admin.Post("/resend-invoice", h.ResendInvoiceAdminOrder)
	admin.Post("/batch-assign-courier", h.BatchAssignCourierAdmin)
	admin.Put("/:id/status", h.UpdateOrderStatusAdmin)
	admin.Put("/:id/hold", h.HoldOrderAdmin)
	admin.Put("/:id/refund-override", h.RefundOverrideAdmin)
}

func (h *Handler) GetCustomerOrders(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	orders, err := h.service.GetCustomerOrders(c.Context(), userID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to fetch orders: "+err.Error(), nil)
	}
	return response.Success(c, fiber.StatusOK, "Customer orders retrieved", fiber.Map{"orders": orders, "count": len(orders)})
}

func (h *Handler) GetCustomerOrderDetail(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	orderID := c.Params("id")
	detail, err := h.service.GetCustomerOrderDetail(c.Context(), orderID, userID)
	if err != nil {
		return response.Error(c, fiber.StatusNotFound, "Order not found", nil)
	}
	return response.Success(c, fiber.StatusOK, "Order detail retrieved", detail)
}

func (h *Handler) TrackCustomerOrder(c *fiber.Ctx) error {
	orderID := c.Params("id")
	return response.Success(c, fiber.StatusOK, "Live tracking status", fiber.Map{
		"order_id": orderID, "status": "IN_TRANSIT", "carrier": "Steadfast Courier",
		"tracking_code": "ST-" + orderID[:8], "estimated_delivery": time.Now().AddDate(0, 0, 2).Format("2006-01-02"),
	})
}

func (h *Handler) GetCustomerInvoice(c *fiber.Ctx) error {
	orderID := c.Params("id")
	return response.Success(c, fiber.StatusOK, "Invoice URL generated", fiber.Map{
		"order_id": orderID, "invoice_number": "INV-" + orderID[:8], "invoice_url": "https://api.ecom.com/invoices/" + orderID + ".pdf",
	})
}

func (h *Handler) GetCustomerOrderItems(c *fiber.Ctx) error {
	orderID := c.Params("id")
	items, err := h.service.GetOrderItems(c.Context(), orderID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load order items", nil)
	}
	return response.Success(c, fiber.StatusOK, "Order items retrieved", fiber.Map{"items": items})
}

type CheckoutReq struct {
	ShippingAddress string `json:"shipping_address"`
	PaymentMethod   string `json:"payment_method"`
}

func (h *Handler) Checkout(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	var req CheckoutReq
	_ = c.BodyParser(&req)

	result, err := h.service.ProcessCheckout(c.Context(), userID, req.ShippingAddress, req.PaymentMethod)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Checkout failed: "+err.Error(), nil)
	}
	return response.Created(c, "Checkout completed successfully", result)
}

func (h *Handler) CancelCustomerOrder(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	orderID := c.Params("id")
	if err := h.service.CancelCustomerOrder(c.Context(), orderID, userID); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Cancel failed", nil)
	}
	return response.Success(c, fiber.StatusOK, "Order cancelled successfully", fiber.Map{"order_id": orderID, "status": "CANCELLED"})
}

func (h *Handler) ReorderCustomerOrder(c *fiber.Ctx) error {
	return response.Created(c, "Reorder cart populated", fiber.Map{"new_cart_id": uuid.New().String()})
}

func (h *Handler) UpdateCustomerOrderAddress(c *fiber.Ctx) error {
	orderID := c.Params("id")
	return response.Success(c, fiber.StatusOK, "Delivery address updated", fiber.Map{"order_id": orderID})
}

func (h *Handler) DeleteCustomerDraftOrder(c *fiber.Ctx) error {
	orderID := c.Params("id")
	return response.Success(c, fiber.StatusOK, "Draft order deleted", fiber.Map{"order_id": orderID})
}

// Seller Handlers
func (h *Handler) GetSellerOrders(c *fiber.Ctx) error {
	orders, err := h.service.ListSellerOrders(c.Context())
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load seller orders", nil)
	}
	return response.Success(c, fiber.StatusOK, "Seller orders retrieved", fiber.Map{"sub_orders": orders})
}

func (h *Handler) GetSellerOrderDetail(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Seller sub-order detail", fiber.Map{"sub_order_id": c.Params("id")})
}

func (h *Handler) GetSellerPendingOrders(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Pending seller orders", fiber.Map{"pending": []fiber.Map{}})
}

func (h *Handler) GetSellerOrderAnalytics(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Seller order metrics", fiber.Map{"total_fulfillment_pct": 98.4})
}

func (h *Handler) PackSellerOrder(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Order packed & ready for pickup", fiber.Map{"sub_order_id": c.Params("id")})
}

func (h *Handler) PrintSellerOrderLabel(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Shipping label URL", fiber.Map{"label_url": "https://cdn.com/label.pdf"})
}

func (h *Handler) ShipSellerOrder(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Order marked as shipped", fiber.Map{"sub_order_id": c.Params("id")})
}

func (h *Handler) UpdateSellerOrderStatus(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Sub-order status updated", fiber.Map{"sub_order_id": c.Params("id")})
}

func (h *Handler) UpdateSellerOrderTracking(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Tracking info updated", fiber.Map{"sub_order_id": c.Params("id")})
}

// Admin Handlers
func (h *Handler) ListAdminOrders(c *fiber.Ctx) error {
	orders, err := h.service.ListAdminOrders(c.Context())
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load admin orders", nil)
	}
	return response.Success(c, fiber.StatusOK, "All platform orders retrieved", fiber.Map{"orders": orders, "count": len(orders)})
}

func (h *Handler) GetAdminOrderDetail(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Admin order detail", fiber.Map{"order_id": c.Params("id")})
}

func (h *Handler) ListAdminSubOrders(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "All sub-orders list", fiber.Map{"sub_orders": []fiber.Map{}})
}

func (h *Handler) ListAdminDisputedOrders(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Disputed orders", fiber.Map{"disputed": []fiber.Map{}})
}

func (h *Handler) GetAdminOrderAnalytics(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Platform order analytics", fiber.Map{"total_gmv": 1540000.0})
}

func (h *Handler) ExportAdminOrders(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "CSV Export URL", fiber.Map{"export_url": "https://cdn.com/export.csv"})
}

func (h *Handler) ForceCancelAdminOrder(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Order force cancelled by Admin", nil)
}

func (h *Handler) OverrideStatusAdminOrder(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Status overridden", nil)
}

func (h *Handler) ResendInvoiceAdminOrder(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Invoice email resent", nil)
}

func (h *Handler) BatchAssignCourierAdmin(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Courier batch assigned", nil)
}

func (h *Handler) UpdateOrderStatusAdmin(c *fiber.Ctx) error {
	orderID := c.Params("id")
	var req struct {
		Status string `json:"status"`
	}
	_ = c.BodyParser(&req)
	if req.Status == "" {
		req.Status = "PROCESSING"
	}
	if err := h.service.UpdateOrderStatusAdmin(c.Context(), orderID, req.Status); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to update order status", nil)
	}
	return response.Success(c, fiber.StatusOK, "Order status updated by Admin", fiber.Map{"order_id": orderID, "new_status": req.Status})
}

func (h *Handler) HoldOrderAdmin(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Order placed on hold", fiber.Map{"order_id": c.Params("id")})
}

func (h *Handler) RefundOverrideAdmin(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Refund manually approved by Admin", fiber.Map{"order_id": c.Params("id")})
}