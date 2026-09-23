package promotion

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
	router.Get("/promotions/flash-sales", h.GetActiveFlashSales)
	router.Post("/promotions/validate-coupon", authMiddleware, h.ValidateCoupon)

	admin := router.Group("/admin/coupons", authMiddleware)
	admin.Post("/", h.CreateCoupon)
}

func (h *Handler) GetActiveFlashSales(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Active flash sales retrieved", fiber.Map{
		"title":       "Mega Cyber Monday Flash Sale",
		"ends_at":     time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		"discount_pct": "30%",
		"items":       []fiber.Map{},
	})
}

type ValidateCouponReq struct {
	CouponCode  string  `json:"coupon_code"`
	OrderAmount float64 `json:"order_amount"`
}

func (h *Handler) ValidateCoupon(c *fiber.Ctx) error {
	var req ValidateCouponReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}
	if req.CouponCode == "" {
		return response.ValidationError(c, map[string]string{"coupon_code": "code is required"})
	}

	// Mock verification
	if req.CouponCode == "DARAZ20" || req.CouponCode == "MEESHO100" {
		discount := 100.00
		if req.OrderAmount < 500 {
			return response.BadRequest(c, "Minimum order amount for this coupon is 500 BDT")
		}
		return response.Success(c, fiber.StatusOK, "Coupon applied successfully", fiber.Map{
			"coupon_code":     req.CouponCode,
			"discount_amount": discount,
			"final_amount":    req.OrderAmount - discount,
		})
	}

	return response.BadRequest(c, "Invalid or expired coupon code")
}

type CreateCouponReq struct {
	Code              string    `json:"code"`
	DiscountType      string    `json:"discount_type"` // PERCENTAGE, FIXED_AMOUNT
	DiscountValue     float64   `json:"discount_value"`
	MinOrderAmount    float64   `json:"min_order_amount"`
	MaxDiscountAmount float64   `json:"max_discount_amount"`
	UsageLimit        int       `json:"usage_limit"`
	ExpiresAt         time.Time `json:"expires_at"`
}

func (h *Handler) CreateCoupon(c *fiber.Ctx) error {
	var req CreateCouponReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid payload: "+err.Error())
	}
	return response.Created(c, "Coupon created successfully", fiber.Map{
		"coupon": req,
	})
}
