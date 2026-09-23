-- ════════════════════════════════════════════════════════════
-- Auth Domain SQL Queries (used by sqlc to generate Go code)
-- ════════════════════════════════════════════════════════════

-- ── User Queries ─────────────────────────────────────────────

-- name: CreateUser :one
INSERT INTO users (
    phone, email, password_hash, full_name, role, status,
    phone_verified, email_verified, referral_code, referred_by_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
) RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1 AND status != 'BLOCKED';

-- name: GetUserByPhone :one
SELECT * FROM users WHERE phone = $1 LIMIT 1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1 LIMIT 1;

-- name: GetUserByGoogleID :one
SELECT * FROM users WHERE google_id = $1 LIMIT 1;

-- name: GetUserByReferralCode :one
SELECT * FROM users WHERE referral_code = $1 LIMIT 1;

-- name: UpdateUserProfile :one
UPDATE users SET
    full_name = COALESCE(sqlc.narg(full_name), full_name),
    email = COALESCE(sqlc.narg(email), email),
    profile_picture_url = COALESCE(sqlc.narg(profile_picture_url), profile_picture_url),
    preferred_language = COALESCE(sqlc.narg(preferred_language), preferred_language),
    date_of_birth = COALESCE(sqlc.narg(date_of_birth), date_of_birth),
    gender = COALESCE(sqlc.narg(gender), gender),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateUserPassword :exec
UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1;

-- name: SetUserEmailVerified :exec
UPDATE users SET email_verified = TRUE, status = 'ACTIVE', updated_at = now() WHERE id = $1;

-- name: SetUserPhoneVerified :exec
UPDATE users SET phone_verified = TRUE, updated_at = now() WHERE id = $1;

-- name: LinkGoogleAccount :exec
UPDATE users SET google_id = $2, updated_at = now() WHERE id = $1;

-- name: UpdateUserRole :exec
UPDATE users SET role = $2, updated_at = now() WHERE id = $1;

-- name: BlockUser :exec
UPDATE users SET status = 'BLOCKED', updated_at = now() WHERE id = $1;

-- name: UnblockUser :exec
UPDATE users SET status = 'ACTIVE', updated_at = now() WHERE id = $1;

-- name: IncrementLoginAttempts :one
UPDATE users SET
    login_attempt_count = login_attempt_count + 1,
    locked_until = CASE WHEN login_attempt_count + 1 >= 10 THEN now() + interval '30 minutes' ELSE locked_until END,
    updated_at = now()
WHERE id = $1
RETURNING login_attempt_count, locked_until;

-- name: ResetLoginAttempts :exec
UPDATE users SET login_attempt_count = 0, locked_until = NULL, last_login_at = now(), updated_at = now() WHERE id = $1;

-- name: UpdateLastLogin :exec
UPDATE users SET last_login_at = now(), updated_at = now() WHERE id = $1;

-- name: CheckPhoneExists :one
SELECT EXISTS(SELECT 1 FROM users WHERE phone = $1) AS exists;

-- name: CheckEmailExists :one
SELECT EXISTS(SELECT 1 FROM users WHERE email = $1) AS exists;

-- ── OTP Queries ───────────────────────────────────────────────

-- name: CreateOTP :one
INSERT INTO otp_requests (phone, otp_hash, purpose, metadata, expires_at, ip_address)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetLatestValidOTP :one
SELECT * FROM otp_requests
WHERE phone = $1 AND purpose = $2 AND used = FALSE AND expires_at > now()
ORDER BY created_at DESC
LIMIT 1;

-- name: IncrementOTPAttempts :one
UPDATE otp_requests SET attempt_count = attempt_count + 1 WHERE id = $1 RETURNING attempt_count;

-- name: MarkOTPUsed :exec
UPDATE otp_requests SET used = TRUE WHERE id = $1;

-- name: CountOTPSentInWindow :one
SELECT COUNT(*) FROM otp_requests
WHERE phone = $1 AND created_at > now() - interval '1 hour';

-- name: DeleteExpiredOTPs :exec
DELETE FROM otp_requests WHERE expires_at < now() AND used = FALSE;

-- ── Session Queries ───────────────────────────────────────────

-- name: CreateSession :one
INSERT INTO sessions (user_id, refresh_token_hash, ip_address, user_agent, device_name, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetSessionByTokenHash :one
SELECT * FROM sessions
WHERE refresh_token_hash = $1 AND is_revoked = FALSE AND expires_at > now();

-- name: GetSessionByID :one
SELECT * FROM sessions WHERE id = $1 AND is_revoked = FALSE;

-- name: UpdateSessionToken :exec
UPDATE sessions SET
    refresh_token_hash = $2,
    last_used_at = now(),
    expires_at = $3
WHERE id = $1;

-- name: RevokeSession :exec
UPDATE sessions SET is_revoked = TRUE WHERE id = $1;

-- name: RevokeAllUserSessions :exec
UPDATE sessions SET is_revoked = TRUE WHERE user_id = $1 AND is_revoked = FALSE;

-- name: GetUserActiveSessions :many
SELECT * FROM sessions WHERE user_id = $1 AND is_revoked = FALSE AND expires_at > now()
ORDER BY last_used_at DESC;

-- name: CleanExpiredSessions :exec
DELETE FROM sessions WHERE expires_at < now() OR is_revoked = TRUE;

-- ── OAuth Queries ─────────────────────────────────────────────

-- name: UpsertOAuthProvider :one
INSERT INTO oauth_providers (user_id, provider, provider_id, access_token, refresh_token, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (provider, provider_id) DO UPDATE SET
    access_token = EXCLUDED.access_token,
    refresh_token = EXCLUDED.refresh_token,
    expires_at = EXCLUDED.expires_at,
    updated_at = now()
RETURNING *;

-- name: GetOAuthProvider :one
SELECT * FROM oauth_providers WHERE provider = $1 AND provider_id = $2;

-- ── Address Queries ───────────────────────────────────────────

-- name: CreateAddress :one
INSERT INTO addresses (user_id, label, recipient_name, recipient_phone, address_line1, address_line2, city, district, upazila, zip_code, is_default)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetUserAddresses :many
SELECT * FROM addresses WHERE user_id = $1 ORDER BY is_default DESC, created_at DESC;

-- name: GetAddressByID :one
SELECT * FROM addresses WHERE id = $1 AND user_id = $2;

-- name: UpdateAddress :one
UPDATE addresses SET
    label = COALESCE(sqlc.narg(label), label),
    recipient_name = COALESCE(sqlc.narg(recipient_name), recipient_name),
    recipient_phone = COALESCE(sqlc.narg(recipient_phone), recipient_phone),
    address_line1 = COALESCE(sqlc.narg(address_line1), address_line1),
    address_line2 = COALESCE(sqlc.narg(address_line2), address_line2),
    city = COALESCE(sqlc.narg(city), city),
    district = COALESCE(sqlc.narg(district), district),
    upazila = COALESCE(sqlc.narg(upazila), upazila),
    zip_code = COALESCE(sqlc.narg(zip_code), zip_code),
    updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: SetDefaultAddress :exec
WITH reset AS (
    UPDATE addresses SET is_default = FALSE WHERE user_id = $1
)
UPDATE addresses SET is_default = TRUE WHERE id = $2 AND user_id = $1;

-- name: DeleteAddress :exec
DELETE FROM addresses WHERE id = $1 AND user_id = $2;

-- ── Audit Log Queries ─────────────────────────────────────────

-- name: CreateAuditLog :one
INSERT INTO audit_logs (actor_id, actor_role, action, entity_type, entity_id, old_value, new_value, reason, ip_address, user_agent)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetAuditLogs :many
SELECT * FROM audit_logs
WHERE entity_type = $1 AND entity_id = $2
ORDER BY timestamp DESC
LIMIT $3 OFFSET $4;
