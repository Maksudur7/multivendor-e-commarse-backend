-- ════════════════════════════════════════════════════════════
-- User Domain SQL Queries (used by sqlc to generate Go code)
-- ════════════════════════════════════════════════════════════

-- name: GetUserByID :one
SELECT id, email, phone, full_name, avatar_url, role, status, is_email_verified, is_phone_verified, kyc_status, default_shipping_address_id, created_at, updated_at
FROM users
WHERE id = $1 LIMIT 1;

-- name: UpdateUserProfile :one
UPDATE users
SET 
  full_name = COALESCE(NULLIF($2, ''), full_name),
  avatar_url = COALESCE(NULLIF($3, ''), avatar_url),
  updated_at = NOW()
WHERE id = $1
RETURNING id, email, phone, full_name, avatar_url, role, status, updated_at;

-- name: SubmitKYCDocument :one
INSERT INTO kyc_documents (
  user_id, document_type, document_number, front_image_url, back_image_url, selfie_image_url, status
) VALUES (
  $1, $2, $3, $4, $5, $6, 'PENDING'
)
ON CONFLICT (user_id) DO UPDATE SET
  document_type = EXCLUDED.document_type,
  document_number = EXCLUDED.document_number,
  front_image_url = EXCLUDED.front_image_url,
  back_image_url = EXCLUDED.back_image_url,
  selfie_image_url = EXCLUDED.selfie_image_url,
  status = 'PENDING',
  rejection_reason = NULL,
  updated_at = NOW()
RETURNING id, user_id, document_type, status, created_at;

-- name: ReviewKYCDocument :one
UPDATE kyc_documents
SET 
  status = $2,
  rejection_reason = $3,
  verified_by = $4,
  verified_at = NOW(),
  updated_at = NOW()
WHERE user_id = $1
RETURNING id, user_id, status, rejection_reason, verified_at;

-- name: UpdateUserKYCStatus :exec
UPDATE users
SET kyc_status = $2, updated_at = NOW()
WHERE id = $1;

-- name: GetUserAddresses :many
SELECT id, user_id, recipient_name, recipient_phone, address_line1, address_line2, division, district, upazila, postal_code, is_default, label, created_at
FROM user_addresses
WHERE user_id = $1
ORDER BY is_default DESC, created_at DESC;

-- name: CreateUserAddress :one
INSERT INTO user_addresses (
  user_id, recipient_name, recipient_phone, address_line1, address_line2, division, district, upazila, postal_code, is_default, label
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
RETURNING id, user_id, recipient_name, recipient_phone, address_line1, division, district, upazila, is_default;

-- name: SetDefaultAddress :exec
UPDATE user_addresses
SET is_default = CASE WHEN id = $2 THEN true ELSE false END
WHERE user_id = $1;

-- name: DeleteUserAddress :exec
DELETE FROM user_addresses
WHERE id = $1 AND user_id = $2;
