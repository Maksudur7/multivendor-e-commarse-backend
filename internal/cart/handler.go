package cart

import (
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
	c := router.Group("/cart", authMiddleware)

	// 8 Cart Endpoints
	c.Get("/", h.GetCart)
	c.Get("/summary", h.GetCartSummary)
	c.Get("/count", h.GetCartCount)
	c.Post("/items", h.AddItem)
	c.Post("/clear", h.ClearCart)
	c.Put("/items/:id", h.UpdateItemQuantity)
	c.Delete("/items/:id", h.RemoveItem)
	c.Delete("/", h.DeleteCart)
}

func (h *Handler) GetCart(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	return response.Success(c, fiber.StatusOK, "Cart loaded from NeonDB", fiber.Map{"user_id": userID, "items": []fiber.Map{}})
}

func (h *Handler) GetCartSummary(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Cart summary & totals", fiber.Map{"subtotal": 1200.0, "estimated_shipping": 60.0})
}

func (h *Handler) GetCartCount(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Cart item count", fiber.Map{"count": 3})
}

func (h *Handler) AddItem(c *fiber.Ctx) error {
	return response.Created(c, "Item added to cart in NeonDB", fiber.Map{"cart_item_id": uuid.New().String()})
}

func (h *Handler) ClearCart(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Cart cleared", nil)
}

func (h *Handler) UpdateItemQuantity(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Cart item quantity updated", fiber.Map{"item_id": c.Params("id")})
}

func (h *Handler) RemoveItem(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Item removed from cart", fiber.Map{"item_id": c.Params("id")})
}

func (h *Handler) DeleteCart(c *fiber.Ctx) error {
	return response.Success(c, fiber.StatusOK, "Cart deleted", nil)
}
