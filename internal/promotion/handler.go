package promotion

import (
	"strings"
	"time"

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
	router.Get("/promotions/flash-sales", h.GetActiveFlashSales)
	router.Post("/promotions/validate-coupon", authMiddleware, h.ValidateCoupon)

	admin := router.Group("/admin/coupons", authMiddleware)
	admin.Get("/", h.ListCouponsAdmin)
	admin.Post("/", h.CreateCoupon)
}

func (h *Handler) GetActiveFlashSales(c *fiber.Ctx) error {
	sales, err := h.service.GetActiveFlashSales(c.Context())
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load flash sales: "+err.Error(), nil)
	}
	return response.Success(c, fiber.StatusOK, "Active flash sales retrieved", fiber.Map{
		"flash_sales": sales, "count": len(sales),
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
	req.CouponCode = strings.ToUpper(strings.TrimSpace(req.CouponCode))
	if req.CouponCode == "" {
		return response.ValidationError(c, map[string]string{"coupon_code": "code is required"})
	}

	res, err := h.service.ValidateCoupon(c.Context(), req.CouponCode, req.OrderAmount)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	return response.Success(c, fiber.StatusOK, "Coupon applied successfully", res)
}

type CreateCouponReq struct {
	Code           string    `json:"code"`
	DiscountType   string    `json:"discount_type"`
	DiscountValue  float64   `json:"discount_value"`
	MinOrderAmount float64   `json:"min_order_amount"`
	ExpiresAt      time.Time `json:"expires_at"`
}

func (h *Handler) ListCouponsAdmin(c *fiber.Ctx) error {
	coupons, err := h.service.ListCouponsAdmin(c.Context())
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load coupons", nil)
	}
	return response.Success(c, fiber.StatusOK, "Admin coupons list", fiber.Map{"coupons": coupons, "count": len(coupons)})
}

func (h *Handler) CreateCoupon(c *fiber.Ctx) error {
	var req CreateCouponReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid payload: "+err.Error())
	}
	req.Code = strings.ToUpper(strings.TrimSpace(req.Code))
	if req.Code == "" {
		return response.ValidationError(c, map[string]string{"code": "code is required"})
	}

	userID := c.Locals("user_id").(string)
	couponID, err := h.service.CreateCoupon(c.Context(), req.Code, req.DiscountType, req.DiscountValue, req.MinOrderAmount, userID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to create coupon: "+err.Error(), nil)
	}

	return response.Created(c, "Coupon created successfully", fiber.Map{
		"coupon_id": couponID, "code": req.Code, "discount_type": req.DiscountType, "discount_value": req.DiscountValue,
	})
}
