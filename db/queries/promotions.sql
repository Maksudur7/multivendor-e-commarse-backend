-- ════════════════════════════════════════════════════════════
-- Promotion Domain SQL Queries
-- ════════════════════════════════════════════════════════════

-- name: CreateCoupon :one
INSERT INTO coupons (
  code, discount_type, discount_value, min_order_amount, max_discount_amount, usage_limit, expires_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7
)
RETURNING id, code, discount_type, discount_value, expires_at;

-- name: ValidateCoupon :one
SELECT id, code, discount_type, discount_value, min_order_amount, max_discount_amount, usage_limit, used_count, expires_at, is_active
FROM coupons
WHERE code = $1 AND is_active = true AND expires_at > NOW() LIMIT 1;
