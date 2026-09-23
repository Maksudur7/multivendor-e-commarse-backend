-- ════════════════════════════════════════════════════════════
-- Payment Domain SQL Queries
-- ════════════════════════════════════════════════════════════

-- name: CreatePaymentTransaction :one
INSERT INTO payment_transactions (
  master_order_id, transaction_reference, payment_gateway, amount, currency, status, gateway_response
) VALUES (
  $1, $2, $3, $4, 'BDT', 'PENDING', $5
)
RETURNING id, master_order_id, transaction_reference, payment_gateway, amount, status, created_at;

-- name: UpdatePaymentStatus :one
UPDATE payment_transactions
SET status = $2, gateway_response = $3, updated_at = NOW()
WHERE transaction_reference = $1
RETURNING id, master_order_id, transaction_reference, status;

-- name: CreateEscrowRecord :one
INSERT INTO escrow_holdings (
  master_order_id, sub_order_id, vendor_id, reseller_id, amount_held, status, auto_release_at
) VALUES (
  $1, $2, $3, $4, $5, 'HELD', NOW() + INTERVAL '7 days'
)
RETURNING id, sub_order_id, vendor_id, amount_held, status, auto_release_at;

-- name: ReleaseEscrow :one
UPDATE escrow_holdings
SET status = 'RELEASED', released_at = NOW(), updated_at = NOW()
WHERE id = $1
RETURNING id, sub_order_id, vendor_id, amount_held, status, released_at;

-- name: CreateRefundRecord :one
INSERT INTO refunds (
  master_order_id, sub_order_id, customer_id, amount, reason, status
) VALUES (
  $1, $2, $3, $4, $5, 'PROCESSING'
)
RETURNING id, master_order_id, amount, status, created_at;
