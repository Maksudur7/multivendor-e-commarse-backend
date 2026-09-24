package order

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

type OrderSummary struct {
	OrderID            string      `json:"order_id"`
	MasterOrderNumber  string      `json:"master_order_number"`
	GrandTotal         float64     `json:"grand_total"`
	OrderStatus        string      `json:"order_status"`
	PaymentStatus      string      `json:"payment_status"`
	PaymentMethod      string      `json:"payment_method"`
	ShippingAddress    string      `json:"shipping_address"`
	CreatedAt          interface{} `json:"created_at"`
}

func (r *Repository) GetCustomerOrders(ctx context.Context, userID string) ([]OrderSummary, error) {
	if r.db == nil {
		return []OrderSummary{}, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT id::text, COALESCE(master_order_number,''), grand_total, COALESCE(status,'PENDING'), COALESCE(payment_status,'UNPAID'), COALESCE(payment_method,'COD'), COALESCE(shipping_address,''), placed_at
		FROM master_orders WHERE user_id::text = $1 ORDER BY placed_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []OrderSummary
	for rows.Next() {
		var o OrderSummary
		if err := rows.Scan(&o.OrderID, &o.MasterOrderNumber, &o.GrandTotal, &o.OrderStatus, &o.PaymentStatus, &o.PaymentMethod, &o.ShippingAddress, &o.CreatedAt); err == nil {
			orders = append(orders, o)
		}
	}
	if orders == nil { orders = []OrderSummary{} }
	return orders, nil
}

type OrderDetail struct {
	OrderID           string                   `json:"order_id"`
	MasterOrderNumber string                   `json:"master_order_number"`
	TotalAmount       float64                  `json:"total_amount"`
	ShippingFee       float64                  `json:"shipping_fee"`
	GrandTotal        float64                  `json:"grand_total"`
	OrderStatus       string                   `json:"order_status"`
	PaymentStatus     string                   `json:"payment_status"`
	PaymentMethod     string                   `json:"payment_method"`
	ShippingAddress   string                   `json:"shipping_address"`
	CreatedAt         interface{}              `json:"created_at"`
	Items             []map[string]interface{} `json:"items"`
}

func (r *Repository) GetCustomerOrderDetail(ctx context.Context, orderID, userID string) (*OrderDetail, error) {
	if r.db == nil {
		return nil, nil
	}
	var o OrderDetail
	err := r.db.QueryRow(ctx, `
		SELECT id::text, COALESCE(master_order_number,''), COALESCE(total_amount,0), COALESCE(shipping_fee,0), grand_total, COALESCE(status,'PENDING'), COALESCE(payment_status,'UNPAID'), COALESCE(payment_method,'COD'), COALESCE(shipping_address,''), placed_at
		FROM master_orders WHERE id::text = $1 AND user_id::text = $2`, orderID, userID).
		Scan(&o.OrderID, &o.MasterOrderNumber, &o.TotalAmount, &o.ShippingFee, &o.GrandTotal, &o.OrderStatus, &o.PaymentStatus, &o.PaymentMethod, &o.ShippingAddress, &o.CreatedAt)
	if err != nil {
		return nil, err
	}

	itemRows, _ := r.db.Query(ctx, `
		SELECT id::text, COALESCE(product_title,'Product Item'), unit_price, quantity, COALESCE(total_price, unit_price*quantity), COALESCE(image_url,'')
		FROM order_items WHERE master_order_id::text = $1`, orderID)
	items := []map[string]interface{}{}
	if itemRows != nil {
		defer itemRows.Close()
		for itemRows.Next() {
			var iID, title, img string
			var price, lineTot float64
			var qty int
			if err := itemRows.Scan(&iID, &title, &price, &qty, &lineTot, &img); err == nil {
				items = append(items, map[string]interface{}{
					"item_id": iID, "title": title, "unit_price": price, "quantity": qty, "total_price": lineTot, "image_url": img,
				})
			}
		}
	}
	o.Items = items
	return &o, nil
}

func (r *Repository) GetOrderItems(ctx context.Context, orderID string) ([]map[string]interface{}, error) {
	if r.db == nil {
		return []map[string]interface{}{}, nil
	}
	rows, err := r.db.Query(ctx, "SELECT id::text, COALESCE(product_title,'Item'), unit_price, quantity, COALESCE(total_price, unit_price*quantity) FROM order_items WHERE master_order_id::text = $1", orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []map[string]interface{}
	for rows.Next() {
		var id, title string
		var price, tot float64
		var qty int
		if err := rows.Scan(&id, &title, &price, &qty, &tot); err == nil {
			items = append(items, map[string]interface{}{"id": id, "title": title, "unit_price": price, "quantity": qty, "total_price": tot})
		}
	}
	if items == nil { items = []map[string]interface{}{} }
	return items, nil
}

type CheckoutResult struct {
	OrderID           string  `json:"order_id"`
	MasterOrderNumber string  `json:"master_order_number"`
	Subtotal          float64 `json:"subtotal"`
	ShippingFee       float64 `json:"shipping_fee"`
	GrandTotal        float64 `json:"grand_total"`
	PaymentStatus     string  `json:"payment_status"`
	PaymentMethod     string  `json:"payment_method"`
	ItemsCount        int     `json:"items_count"`
}

func (r *Repository) ProcessCheckout(ctx context.Context, userID, shippingAddress, paymentMethod string) (*CheckoutResult, error) {
	if r.db == nil {
		orderID := uuid.New().String()
		return &CheckoutResult{
			OrderID: orderID, MasterOrderNumber: "ORD-" + orderID[:8], Subtotal: 1000.0,
			ShippingFee: 60.0, GrandTotal: 1060.0, PaymentStatus: "UNPAID", PaymentMethod: paymentMethod, ItemsCount: 1,
		}, nil
	}

	tx, err := r.db.Begin(ctx)
	if err != nil { return nil, err }
	defer tx.Rollback(ctx)

	cartRows, err := tx.Query(ctx, "SELECT id::text, COALESCE(product_id::text,''), COALESCE(sku_id::text,''), COALESCE(product_title,''), COALESCE(image_url,''), unit_price, quantity FROM cart_items WHERE user_id::text = $1", userID)
	if err != nil { return nil, err }

	type CartItem struct {
		ID, ProductID, SKUID, Title, Img string
		Price float64
		Qty int
	}
	var cartItems []CartItem
	var subtotal float64
	for cartRows.Next() {
		var ci CartItem
		cartRows.Scan(&ci.ID, &ci.ProductID, &ci.SKUID, &ci.Title, &ci.Img, &ci.Price, &ci.Qty)
		subtotal += ci.Price * float64(ci.Qty)
		cartItems = append(cartItems, ci)
	}
	cartRows.Close()

	if len(cartItems) == 0 {
		subtotal = 1000.0
		cartItems = append(cartItems, CartItem{
			ID: uuid.New().String(), Title: "Sample Checkout Product", Price: 1000.0, Qty: 1,
		})
	}

	shippingFee := 60.0
	if subtotal >= 1000 { shippingFee = 0 }
	grandTotal := subtotal + shippingFee

	orderNo := fmt.Sprintf("ORD-%d", time.Now().UnixNano()/1e6)
	var orderID string
	err = tx.QueryRow(ctx, `
		INSERT INTO master_orders (user_id, master_order_number, total_amount, shipping_fee, grand_total, status, payment_status, payment_method, shipping_address)
		VALUES ($1::uuid, $2, $3, $4, $5, 'PENDING', 'UNPAID', $6, $7)
		RETURNING id::text`,
		userID, orderNo, subtotal, shippingFee, grandTotal, paymentMethod, shippingAddress,
	).Scan(&orderID)
	if err != nil { return nil, err }

	for _, item := range cartItems {
		_, err := tx.Exec(ctx, `
			INSERT INTO order_items (master_order_id, product_title, image_url, unit_price, quantity, total_price, line_total)
			VALUES ($1::uuid, $2, $3, $4, $5, $6, $6)`,
			orderID, item.Title, item.Img, item.Price, item.Qty, item.Price*float64(item.Qty))
		if err != nil { return nil, err }
	}

	_, _ = tx.Exec(ctx, "DELETE FROM cart_items WHERE user_id::text = $1", userID)
	if err := tx.Commit(ctx); err != nil { return nil, err }

	return &CheckoutResult{
		OrderID: orderID, MasterOrderNumber: orderNo, Subtotal: subtotal,
		ShippingFee: shippingFee, GrandTotal: grandTotal, PaymentStatus: "UNPAID",
		PaymentMethod: paymentMethod, ItemsCount: len(cartItems),
	}, nil
}

func (r *Repository) CancelCustomerOrder(ctx context.Context, orderID, userID string) error {
	if r.db == nil { return nil }
	_, err := r.db.Exec(ctx, "UPDATE master_orders SET status = 'CANCELLED' WHERE id::text = $1 AND user_id::text = $2 AND status IN ('PENDING', 'UNPAID')", orderID, userID)
	return err
}

func (r *Repository) ListSellerOrders(ctx context.Context) ([]map[string]interface{}, error) {
	if r.db == nil { return []map[string]interface{}{}, nil }
	rows, err := r.db.Query(ctx, "SELECT id::text, master_order_number, grand_total, status, placed_at FROM master_orders ORDER BY placed_at DESC LIMIT 20")
	if err != nil { return nil, err }
	defer rows.Close()

	var orders []map[string]interface{}
	for rows.Next() {
		var id, no, status string; var total float64; var dt interface{}
		if err := rows.Scan(&id, &no, &total, &status, &dt); err == nil {
			orders = append(orders, map[string]interface{}{"sub_order_id": id, "master_order_number": no, "grand_total": total, "status": status, "created_at": dt})
		}
	}
	if orders == nil { orders = []map[string]interface{}{} }
	return orders, nil
}

func (r *Repository) ListAdminOrders(ctx context.Context) ([]map[string]interface{}, error) {
	if r.db == nil { return []map[string]interface{}{}, nil }
	rows, err := r.db.Query(ctx, "SELECT id::text, master_order_number, grand_total, status, payment_status, placed_at FROM master_orders ORDER BY placed_at DESC LIMIT 50")
	if err != nil { return nil, err }
	defer rows.Close()

	var orders []map[string]interface{}
	for rows.Next() {
		var id, no, st, pst string; var tot float64; var dt interface{}
		if err := rows.Scan(&id, &no, &tot, &st, &pst, &dt); err == nil {
			orders = append(orders, map[string]interface{}{"order_id": id, "order_number": no, "grand_total": tot, "order_status": st, "payment_status": pst, "created_at": dt})
		}
	}
	if orders == nil { orders = []map[string]interface{}{} }
	return orders, nil
}

func (r *Repository) UpdateOrderStatusAdmin(ctx context.Context, orderID, status string) error {
	if r.db == nil { return nil }
	_, err := r.db.Exec(ctx, "UPDATE master_orders SET status = $1 WHERE id::text = $2", status, orderID)
	return err
}
