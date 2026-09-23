-- ════════════════════════════════════════════════════════════
-- Shipping Domain SQL Queries
-- ════════════════════════════════════════════════════════════

-- name: CreateShipment :one
INSERT INTO shipments (
  sub_order_id, courier_name, tracking_number, consignment_id, delivery_fee, status, waybill_url
) VALUES (
  $1, $2, $3, $4, $5, 'PICKUP_PENDING', $6
)
RETURNING id, sub_order_id, courier_name, tracking_number, status, waybill_url, created_at;

-- name: UpdateShipmentStatus :one
UPDATE shipments
SET status = $2, tracking_history = tracking_history || $3::jsonb, updated_at = NOW()
WHERE tracking_number = $1
RETURNING id, sub_order_id, status, tracking_number;
