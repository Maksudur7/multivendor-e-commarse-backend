package cart

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yourusername/ecom-backend/pkg/response"
)

type Handler struct {
	db      *pgxpool.Pool
	service *Service
}

func NewHandler(db *pgxpool.Pool) *Handler {
	repo := NewRepository(db)
	service := NewService(repo)
	return &Handler{
		db:      db,
		service: service,
	}
}

func (h *Handler) RegisterRoutes(router fiber.Router, authMiddleware fiber.Handler) {
	c := router.Group("/cart", authMiddleware)
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

	result, err := h.service.GetCart(c.Context(), userID)
	if err != nil && h.db != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load cart", nil)
	}

	items := []CartItem{}
	grandTotal := 0.0
	if result != nil {
		items = result.Items
		grandTotal = result.GrandTotal
	}

	return response.Success(c, fiber.StatusOK, "Cart loaded from NeonDB", fiber.Map{
		"user_id": userID, "items": items, "item_count": len(items), "grand_total": grandTotal,
	})
}

func (h *Handler) GetCartSummary(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	result, _ := h.service.GetCartSummary(c.Context(), userID)

	summary := &CartSummaryResult{FreeShippingFrom: 1000}
	if result != nil {
		summary = result
	}

	return response.Success(c, fiber.StatusOK, "Cart summary from NeonDB", fiber.Map{
		"subtotal":           summary.Subtotal,
		"item_count":         summary.ItemCount,
		"estimated_shipping": summary.EstimatedShipping,
		"grand_total":        summary.GrandTotal,
		"free_shipping_from": summary.FreeShippingFrom,
	})
}

func (h *Handler) GetCartCount(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	count, _ := h.service.GetCartCount(c.Context(), userID)
	return response.Success(c, fiber.StatusOK, "Cart count from NeonDB", fiber.Map{"count": count})
}

type AddItemReq struct {
	ProductID    string  `json:"product_id"`
	SKUID        string  `json:"sku_id"`
	ProductTitle string  `json:"product_title"`
	ImageURL     string  `json:"image_url"`
	UnitPrice    float64 `json:"unit_price"`
	Quantity     int     `json:"quantity"`
}

func (h *Handler) AddItem(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	var req AddItemReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body: "+err.Error())
	}
	if req.SKUID == "" && req.ProductID == "" {
		return response.ValidationError(c, map[string]string{"item": "sku_id or product_id is required"})
	}

	cartItemID, _, err := h.service.AddItem(c.Context(), AddItemInput{
		UserID:       userID,
		ProductID:    req.ProductID,
		SKUID:        req.SKUID,
		ProductTitle: req.ProductTitle,
		ImageURL:     req.ImageURL,
		UnitPrice:    req.UnitPrice,
		Quantity:     req.Quantity,
	})
	if err != nil && h.db != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to add item: "+err.Error(), nil)
	}

	qty := req.Quantity
	if qty <= 0 {
		qty = 1
	}

	return response.Created(c, "Item added to cart in NeonDB", fiber.Map{
		"cart_item_id": cartItemID, "quantity": qty,
	})
}

func (h *Handler) ClearCart(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	_ = h.service.ClearCart(c.Context(), userID)
	return response.Success(c, fiber.StatusOK, "Cart cleared", nil)
}

type UpdateQtyReq struct{ Quantity int `json:"quantity"` }

func (h *Handler) UpdateItemQuantity(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	itemID := c.Params("id")
	var req UpdateQtyReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid input: "+err.Error())
	}
	if req.Quantity <= 0 {
		return response.ValidationError(c, map[string]string{"quantity": "must be >= 1"})
	}

	affected, err := h.service.UpdateQuantity(c.Context(), userID, itemID, req.Quantity)
	if err != nil && h.db != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to update quantity", nil)
	}
	if affected == 0 && h.db != nil {
		return response.Error(c, fiber.StatusNotFound, "Cart item not found", nil)
	}
	return response.Success(c, fiber.StatusOK, "Cart item quantity updated", fiber.Map{"item_id": itemID, "new_quantity": req.Quantity})
}

func (h *Handler) RemoveItem(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	itemID := c.Params("id")

	affected, err := h.service.RemoveItem(c.Context(), userID, itemID)
	if err != nil && h.db != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to remove item", nil)
	}
	if affected == 0 && h.db != nil {
		return response.Error(c, fiber.StatusNotFound, "Cart item not found", nil)
	}
	return response.Success(c, fiber.StatusOK, "Item removed from cart", fiber.Map{"item_id": itemID})
}

func (h *Handler) DeleteCart(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	_ = h.service.ClearCart(c.Context(), userID)
	return response.Success(c, fiber.StatusOK, "Cart deleted", nil)
}
