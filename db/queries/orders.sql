-- ════════════════════════════════════════════════════════════
-- Order Domain SQL Queries
-- ════════════════════════════════════════════════════════════

-- name: CreateMasterOrder :one
INSERT INTO master_orders (
  customer_id, reseller_id, affiliate_id, master_order_number, total_amount, shipping_fee, discount_amount, grand_total, payment_status, payment_method, shipping_address
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, 'UNPAID', $9, $10
)
RETURNING id, master_order_number, grand_total, payment_status, created_at;

-- name: CreateSubOrder :one
INSERT INTO sub_orders (
  master_order_id, vendor_id, sub_order_number, item_total, vendor_commission_amount, platform_fee, vendor_payout_amount, status
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, 'PENDING'
)
RETURNING id, master_order_id, vendor_id, sub_order_number, vendor_payout_amount, status;

-- name: CreateOrderItem :one
INSERT INTO order_items (
  sub_order_id, product_id, sku_id, product_title, sku_attributes, unit_wholesale_price, unit_retail_price, reseller_margin_unit, quantity, total_price, total_reseller_profit
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
RETURNING id, sub_order_id, quantity, total_price;

-- name: UpdateSubOrderStatus :one
UPDATE sub_orders
SET status = $2, updated_at = NOW()
WHERE id = $1
RETURNING id, sub_order_number, status, updated_at;

-- name: UpdateMasterOrderPaymentStatus :one
UPDATE master_orders
SET payment_status = $2, updated_at = NOW()
WHERE id = $1
RETURNING id, master_order_number, payment_status;

-- name: GetCustomerOrders :many
SELECT id, master_order_number, grand_total, payment_status, payment_method, created_at
FROM master_orders
WHERE customer_id = $1
ORDER BY created_at DESC;

-- name: GetVendorSubOrders :many
SELECT id, master_order_id, sub_order_number, item_total, vendor_payout_amount, status, created_at
FROM sub_orders
WHERE vendor_id = $1
ORDER BY created_at DESC;
