package cart

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yourusername/ecom-backend/pkg/response"
)

type Handler struct { db *pgxpool.Pool }
func NewHandler(db *pgxpool.Pool) *Handler { return &Handler{db: db} }

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
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Cart loaded", fiber.Map{"user_id": userID, "items": []fiber.Map{}, "total": 0})
	}
	ctx := c.Context()
	rows, err := h.db.Query(ctx, `
		SELECT ci.id::text, ci.product_title, COALESCE(ci.image_url,''), ci.unit_price,
		       ci.quantity, (ci.unit_price * ci.quantity) as line_total,
		       COALESCE(ci.sku_id::text,''), COALESCE(ci.product_id::text,'')
		FROM cart_items ci WHERE ci.user_id::text = $1 ORDER BY ci.added_at DESC`, userID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to load cart", nil)
	}
	defer rows.Close()

	items := []fiber.Map{}
	var grandTotal float64
	for rows.Next() {
		var id, title, img, skuID, productID string
		var price, lineTotal float64
		var qty int
		if err := rows.Scan(&id, &title, &img, &price, &qty, &lineTotal, &skuID, &productID); err != nil { continue }
		grandTotal += lineTotal
		items = append(items, fiber.Map{
			"cart_item_id": id, "product_title": title, "image_url": img,
			"unit_price": price, "quantity": qty, "line_total": lineTotal,
			"sku_id": skuID, "product_id": productID,
		})
	}
	return response.Success(c, fiber.StatusOK, "Cart loaded from NeonDB", fiber.Map{
		"user_id": userID, "items": items, "item_count": len(items), "grand_total": grandTotal,
	})
}

func (h *Handler) GetCartSummary(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Cart summary", fiber.Map{"subtotal": 0, "estimated_shipping": 60})
	}
	ctx := c.Context()
	var subtotal float64; var count int
	h.db.QueryRow(ctx, "SELECT COALESCE(SUM(unit_price*quantity),0), COUNT(*) FROM cart_items WHERE user_id::text = $1", userID).Scan(&subtotal, &count)
	shipping := 60.0
	if subtotal >= 1000 { shipping = 0 }
	return response.Success(c, fiber.StatusOK, "Cart summary from NeonDB", fiber.Map{
		"subtotal": subtotal, "item_count": count, "estimated_shipping": shipping,
		"grand_total": subtotal + shipping, "free_shipping_from": 1000,
	})
}

func (h *Handler) GetCartCount(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Cart count", fiber.Map{"count": 0})
	}
	ctx := c.Context()
	var count int
	h.db.QueryRow(ctx, "SELECT COALESCE(SUM(quantity),0) FROM cart_items WHERE user_id::text = $1", userID).Scan(&count)
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
	ctx := c.Context()
	if req.UnitPrice <= 0 {
		if req.SKUID != "" && h.db != nil {
			_ = h.db.QueryRow(ctx, "SELECT retail_price FROM product_skus WHERE id::text = $1", req.SKUID).Scan(&req.UnitPrice)
		}
		if req.UnitPrice <= 0 {
			req.UnitPrice = 1200.0
		}
	}

	if req.Quantity <= 0 { req.Quantity = 1 }

	if h.db == nil {
		return response.Created(c, "Item added to cart", fiber.Map{"quantity": req.Quantity})
	}

	var cartItemID string
	err := h.db.QueryRow(ctx, `
		INSERT INTO cart_items (user_id, product_id, sku_id, product_title, image_url, unit_price, quantity)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7)
		ON CONFLICT (user_id, sku_id) DO UPDATE SET quantity = cart_items.quantity + EXCLUDED.quantity
		RETURNING id::text`,
		userID, nullIfEmpty(req.ProductID), nullIfEmpty(req.SKUID),
		req.ProductTitle, req.ImageURL, req.UnitPrice, req.Quantity,
	).Scan(&cartItemID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to add item: "+err.Error(), nil)
	}
	return response.Created(c, "Item added to cart in NeonDB", fiber.Map{
		"cart_item_id": cartItemID, "quantity": req.Quantity,
	})
}

func nullIfEmpty(s string) *string {
	if strings.TrimSpace(s) == "" { return nil }
	return &s
}

func (h *Handler) ClearCart(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	if h.db != nil {
		h.db.Exec(c.Context(), "DELETE FROM cart_items WHERE user_id::text = $1", userID)
	}
	return response.Success(c, fiber.StatusOK, "Cart cleared", nil)
}

type UpdateQtyReq struct { Quantity int `json:"quantity"` }

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
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Quantity updated", fiber.Map{"item_id": itemID})
	}
	ctx := c.Context()
	r, err := h.db.Exec(ctx,
		"UPDATE cart_items SET quantity = $1 WHERE id::text = $2 AND user_id::text = $3",
		req.Quantity, itemID, userID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to update quantity", nil)
	}
	if r.RowsAffected() == 0 {
		return response.Error(c, fiber.StatusNotFound, "Cart item not found", nil)
	}
	return response.Success(c, fiber.StatusOK, "Cart item quantity updated", fiber.Map{"item_id": itemID, "new_quantity": req.Quantity})
}

func (h *Handler) RemoveItem(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	itemID := c.Params("id")
	if h.db == nil {
		return response.Success(c, fiber.StatusOK, "Item removed", fiber.Map{"item_id": itemID})
	}
	ctx := c.Context()
	r, err := h.db.Exec(ctx, "DELETE FROM cart_items WHERE id::text = $1 AND user_id::text = $2", itemID, userID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to remove item", nil)
	}
	if r.RowsAffected() == 0 {
		return response.Error(c, fiber.StatusNotFound, "Cart item not found", nil)
	}
	return response.Success(c, fiber.StatusOK, "Item removed from cart", fiber.Map{"item_id": itemID})
}

func (h *Handler) DeleteCart(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	if h.db != nil {
		h.db.Exec(c.Context(), "DELETE FROM cart_items WHERE user_id::text = $1", userID)
	}
	return response.Success(c, fiber.StatusOK, "Cart deleted", nil)
}

