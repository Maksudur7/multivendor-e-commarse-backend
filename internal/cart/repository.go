package cart

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

type CartItem struct {
	ID           string  `json:"cart_item_id"`
	ProductTitle string  `json:"product_title"`
	ImageURL     string  `json:"image_url"`
	UnitPrice    float64 `json:"unit_price"`
	Quantity     int     `json:"quantity"`
	LineTotal    float64 `json:"line_total"`
	SKUID        string  `json:"sku_id"`
	ProductID    string  `json:"product_id"`
}

func (r *Repository) GetUserCart(ctx context.Context, userID string) ([]CartItem, float64, error) {
	if r.db == nil {
		return []CartItem{}, 0, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT ci.id::text, ci.product_title, COALESCE(ci.image_url,''), ci.unit_price,
		       ci.quantity, (ci.unit_price * ci.quantity) as line_total,
		       COALESCE(ci.sku_id::text,''), COALESCE(ci.product_id::text,'')
		FROM cart_items ci WHERE ci.user_id::text = $1 ORDER BY ci.added_at DESC`, userID)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []CartItem
	var grandTotal float64
	for rows.Next() {
		var item CartItem
		if err := rows.Scan(&item.ID, &item.ProductTitle, &item.ImageURL, &item.UnitPrice, &item.Quantity, &item.LineTotal, &item.SKUID, &item.ProductID); err == nil {
			grandTotal += item.LineTotal
			items = append(items, item)
		}
	}
	if items == nil { items = []CartItem{} }
	return items, grandTotal, nil
}

func (r *Repository) GetCartSummary(ctx context.Context, userID string) (float64, int, error) {
	if r.db == nil {
		return 0, 0, nil
	}
	var subtotal float64
	var count int
	err := r.db.QueryRow(ctx, "SELECT COALESCE(SUM(unit_price*quantity),0), COUNT(*) FROM cart_items WHERE user_id::text = $1", userID).Scan(&subtotal, &count)
	return subtotal, count, err
}

func (r *Repository) GetCartCount(ctx context.Context, userID string) (int, error) {
	if r.db == nil {
		return 0, nil
	}
	var count int
	err := r.db.QueryRow(ctx, "SELECT COALESCE(SUM(quantity),0) FROM cart_items WHERE user_id::text = $1", userID).Scan(&count)
	return count, err
}

func (r *Repository) FetchSKUPrice(ctx context.Context, skuID string) float64 {
	if r.db == nil || skuID == "" {
		return 0
	}
	var price float64
	_ = r.db.QueryRow(ctx, "SELECT retail_price FROM product_skus WHERE id::text = $1", skuID).Scan(&price)
	return price
}

func (r *Repository) AddItem(ctx context.Context, userID, productID, skuID, productTitle, imageURL string, unitPrice float64, quantity int) (string, error) {
	if r.db == nil {
		return "", nil
	}
	var cartItemID string
	err := r.db.QueryRow(ctx, `
		INSERT INTO cart_items (user_id, product_id, sku_id, product_title, image_url, unit_price, quantity)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7)
		ON CONFLICT (user_id, sku_id) DO UPDATE SET quantity = cart_items.quantity + EXCLUDED.quantity
		RETURNING id::text`,
		userID, nullIfEmpty(productID), nullIfEmpty(skuID),
		productTitle, imageURL, unitPrice, quantity,
	).Scan(&cartItemID)
	return cartItemID, err
}

func (r *Repository) UpdateQuantity(ctx context.Context, userID, itemID string, quantity int) (int64, error) {
	if r.db == nil {
		return 0, nil
	}
	res, err := r.db.Exec(ctx, "UPDATE cart_items SET quantity = $1 WHERE id::text = $2 AND user_id::text = $3", quantity, itemID, userID)
	if err != nil { return 0, err }
	return res.RowsAffected(), nil
}

func (r *Repository) RemoveItem(ctx context.Context, userID, itemID string) (int64, error) {
	if r.db == nil {
		return 0, nil
	}
	res, err := r.db.Exec(ctx, "DELETE FROM cart_items WHERE id::text = $1 AND user_id::text = $2", itemID, userID)
	if err != nil { return 0, err }
	return res.RowsAffected(), nil
}

func (r *Repository) ClearCart(ctx context.Context, userID string) error {
	if r.db == nil {
		return nil
	}
	_, err := r.db.Exec(ctx, "DELETE FROM cart_items WHERE user_id::text = $1", userID)
	return err
}

func nullIfEmpty(s string) *string {
	if strings.TrimSpace(s) == "" { return nil }
	return &s
}
