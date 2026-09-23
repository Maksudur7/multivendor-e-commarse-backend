-- ════════════════════════════════════════════════════════════
-- Reseller Domain SQL Queries
-- ════════════════════════════════════════════════════════════

-- name: GetResellerByUserID :one
SELECT id, user_id, store_name, store_slug, custom_domain, total_sales_count, total_profit_earned, wallet_balance, created_at
FROM resellers
WHERE user_id = $1 LIMIT 1;

-- name: GetResellerBySlug :one
SELECT id, user_id, store_name, store_slug, custom_domain, created_at
FROM resellers
WHERE store_slug = $1 LIMIT 1;

-- name: CreateResellerStore :one
INSERT INTO resellers (
  user_id, store_name, store_slug, custom_domain
) VALUES (
  $1, $2, $3, $4
)
RETURNING id, user_id, store_name, store_slug, created_at;

-- name: AddProductToResellerCatalog :one
INSERT INTO reseller_catalog (
  reseller_id, product_id, sku_id, wholesale_price, reseller_margin, final_selling_price
) VALUES (
  $1, $2, $3, $4, $5, $6
)
ON CONFLICT (reseller_id, sku_id) DO UPDATE SET
  reseller_margin = EXCLUDED.reseller_margin,
  final_selling_price = EXCLUDED.final_selling_price,
  updated_at = NOW()
RETURNING id, reseller_id, product_id, final_selling_price;

-- name: GetResellerCatalog :many
SELECT rc.id, rc.reseller_id, rc.product_id, rc.sku_id, rc.wholesale_price, rc.reseller_margin, rc.final_selling_price, p.title, p.primary_image_url
FROM reseller_catalog rc
JOIN products p ON rc.product_id = p.id
WHERE rc.reseller_id = $1
ORDER BY rc.created_at DESC;

-- name: RemoveProductFromResellerCatalog :exec
DELETE FROM reseller_catalog
WHERE reseller_id = $1 AND product_id = $2;
