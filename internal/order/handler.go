package order

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
	// Customer orders
	c := router.Group("/orders", authMiddleware)
	c.Post("/checkout", h.Checkout)
	c.Get("/", h.GetCustomerOrders)
	c.Get("/:id", h.GetOrderDetail)

	// Vendor sub-orders
	v := router.Group("/vendor/orders", authMiddleware)
	v.Get("/", h.GetVendorOrders)
	v.Put("/:subOrderId/status", h.UpdateSubOrderStatus)
}

type CheckoutItem struct {
	ProductID      string                 `json:"product_id"`
	SKUID          string                 `json:"sku_id"`
	VendorID       string                 `json:"vendor_id"`
	ProductTitle   string                 `json:"product_title"`
	SKUAttributes  map[string]interface{} `json:"sku_attributes"`
	Quantity       int                    `json:"quantity"`
	WholesalePrice float64                `json:"wholesale_price"`
	RetailPrice    float64                `json:"retail_price"`
	ResellerMargin float64                `json:"reseller_margin"`
}

type CheckoutReq struct {
	ResellerID      string         `json:"reseller_id,omitempty"`
	AffiliateID     string         `json:"affiliate_id,omitempty"`
	Items           []CheckoutItem `json:"items"`
	ShippingAddress string         `json:"shipping_address"`
	PaymentMethod   string         `json:"payment_method"` // bKash, SSLCommerz, Nagad, COD
	CouponCode      string         `json:"coupon_code,omitempty"`
}

func (h *Handler) Checkout(c *fiber.Ctx) error {
	var req CheckoutReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid checkout payload: "+err.Error())
	}
	if len(req.Items) == 0 {
		return response.BadRequest(c, "Cart is empty")
	}
	if req.ShippingAddress == "" || req.PaymentMethod == "" {
		return response.ValidationError(c, map[string]string{
			"required": "shipping_address and payment_method are mandatory",
		})
	}

	customerID := c.Locals("user_id").(string)
	masterOrderNo := fmt.Sprintf("ORD-%d", time.Now().UnixNano()/1e6)

	// 1. Group items by VendorID for Multi-Vendor Sub-Orders
	vendorItems := make(map[string][]CheckoutItem)
	var grandTotal float64

	for _, item := range req.Items {
		itemTotal := (item.RetailPrice + item.ResellerMargin) * float64(item.Quantity)
		grandTotal += itemTotal
		vID := item.VendorID
		if vID == "" {
			vID = "FIRST_PARTY_WAREHOUSE"
		}
		vendorItems[vID] = append(vendorItems[vID], item)
	}

	// 2. Generate sub-orders summary
	subOrdersSummary := []fiber.Map{}
	for vID, items := range vendorItems {
		subOrderNo := fmt.Sprintf("SUB-%s-%d", vID[:4], time.Now().Unix()%10000)
		var subTotal float64
		for _, it := range items {
			subTotal += (it.RetailPrice + it.ResellerMargin) * float64(it.Quantity)
		}
		// Calculate platform commission (e.g. 5%)
		platformFee := subTotal * 0.05
		vendorPayout := subTotal - platformFee

		subOrdersSummary = append(subOrdersSummary, fiber.Map{
			"vendor_id":        vID,
			"sub_order_number": subOrderNo,
			"item_count":       len(items),
			"sub_total":        subTotal,
			"platform_fee":     platformFee,
			"vendor_payout":    vendorPayout,
		})
	}

	return response.Created(c, "Master order & multi-vendor sub-orders created successfully", fiber.Map{
		"master_order_id":     uuid.New().String(),
		"master_order_number": masterOrderNo,
		"customer_id":         customerID,
		"grand_total":         grandTotal,
		"payment_method":      req.PaymentMethod,
		"payment_status":      "UNPAID",
		"sub_orders":          subOrdersSummary,
	})
}

func (h *Handler) GetCustomerOrders(c *fiber.Ctx) error {
	customerID := c.Locals("user_id").(string)
	return response.Success(c, fiber.StatusOK, "Customer order history", fiber.Map{
		"customer_id": customerID,
		"orders":      []fiber.Map{},
	})
}

func (h *Handler) GetOrderDetail(c *fiber.Ctx) error {
	orderID := c.Params("id")
	return response.Success(c, fiber.StatusOK, "Order details retrieved", fiber.Map{
		"master_order_id": orderID,
		"status":          "PROCESSING",
	})
}

func (h *Handler) GetVendorOrders(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Vendor sub-orders retrieved", fiber.Map{
		"sub_orders": []fiber.Map{},
	})
}

type UpdateSubOrderStatusReq struct {
	Status         string `json:"status"` // CONFIRMED, PACKED, SHIPPED, DELIVERED, CANCELLED
	TrackingNumber string `json:"tracking_number,omitempty"`
}

func (h *Handler) UpdateSubOrderStatus(c *fiber.Ctx) error {
	subOrderID := c.Params("subOrderId")
	var req UpdateSubOrderStatusReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}
	return response.Success(c, fiber.StatusOK, "Sub-order status updated", fiber.Map{
		"sub_order_id":    subOrderID,
		"status":          req.Status,
		"tracking_number": req.TrackingNumber,
	})
}
