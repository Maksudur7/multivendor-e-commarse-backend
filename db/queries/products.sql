-- ════════════════════════════════════════════════════════════
-- Product Domain SQL Queries
-- ════════════════════════════════════════════════════════════

-- name: CreateCategory :one
INSERT INTO categories (
  name, slug, parent_id, icon_url, commission_rate
) VALUES (
  $1, $2, $3, $4, $5
)
RETURNING id, name, slug, parent_id, commission_rate, created_at;

-- name: GetCategoryTree :many
SELECT id, name, slug, parent_id, icon_url, commission_rate, created_at
FROM categories
ORDER BY parent_id NULLS FIRST, name ASC;

-- name: CreateProduct :one
INSERT INTO products (
  vendor_id, category_id, brand_id, title, slug, description, short_description, primary_image_url, is_first_party, is_resellable, wholesale_price, retail_price, approval_status
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, 'PENDING'
)
RETURNING id, vendor_id, category_id, title, slug, approval_status, created_at;

-- name: CreateProductSKU :one
INSERT INTO product_skus (
  product_id, sku_code, attributes, wholesale_price, retail_price, stock_quantity, reserved_quantity, image_url
) VALUES (
  $1, $2, $3, $4, $5, $6, 0, $7
)
RETURNING id, product_id, sku_code, stock_quantity;

-- name: GetProductBySlug :one
SELECT p.id, p.vendor_id, p.category_id, p.brand_id, p.title, p.slug, p.description, p.primary_image_url, p.wholesale_price, p.retail_price, p.rating_avg, p.review_count, v.store_name, v.store_slug
FROM products p
LEFT JOIN vendors v ON p.vendor_id = v.id
WHERE p.slug = $1 AND p.approval_status = 'APPROVED' LIMIT 1;

-- name: ListProductsStorefront :many
SELECT id, title, slug, primary_image_url, wholesale_price, retail_price, is_resellable, rating_avg, review_count, sales_count
FROM products
WHERE approval_status = 'APPROVED'
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ApproveProduct :one
UPDATE products
SET approval_status = $2, updated_at = NOW()
WHERE id = $1
RETURNING id, title, approval_status;
