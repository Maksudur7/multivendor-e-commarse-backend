-- ════════════════════════════════════════════════════════════
-- Vendor Domain SQL Queries
-- ════════════════════════════════════════════════════════════

-- name: GetVendorByUserID :one
SELECT id, user_id, store_name, store_slug, logo_url, banner_url, description, commission_rate, verification_status, total_sales_count, total_revenue, rating_avg, created_at
FROM vendors
WHERE user_id = $1 LIMIT 1;

-- name: GetVendorBySlug :one
SELECT id, user_id, store_name, store_slug, logo_url, banner_url, description, commission_rate, verification_status, total_sales_count, rating_avg, created_at
FROM vendors
WHERE store_slug = $1 LIMIT 1;

-- name: CreateVendorProfile :one
INSERT INTO vendors (
  user_id, store_name, store_slug, logo_url, banner_url, description, bank_name, bank_account_number, bank_routing_number, bkash_merchant_number, nagad_merchant_number, verification_status
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'PENDING'
)
RETURNING id, user_id, store_name, store_slug, verification_status, created_at;

-- name: UpdateVendorProfile :one
UPDATE vendors
SET
  store_name = COALESCE(NULLIF($2, ''), store_name),
  logo_url = COALESCE(NULLIF($3, ''), logo_url),
  banner_url = COALESCE(NULLIF($4, ''), banner_url),
  description = COALESCE(NULLIF($5, ''), description),
  bank_name = COALESCE(NULLIF($6, ''), bank_name),
  bank_account_number = COALESCE(NULLIF($7, ''), bank_account_number),
  updated_at = NOW()
WHERE id = $1
RETURNING id, store_name, logo_url, banner_url, description, updated_at;

-- name: VerifyVendor :one
UPDATE vendors
SET
  verification_status = $2,
  commission_rate = COALESCE($3, commission_rate),
  updated_at = NOW()
WHERE id = $1
RETURNING id, store_name, verification_status, commission_rate;

-- name: ListVendorsAdmin :many
SELECT id, user_id, store_name, store_slug, verification_status, commission_rate, total_revenue, rating_avg, created_at
FROM vendors
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;
