-- ════════════════════════════════════════════════════════════
-- Migration 0010: Auth Security Hardening
-- Applied:  2026-09-24
-- Purpose:  Support hashed OTP attempt-counting, session metadata,
--           and password-strength audit fields added in the security rewrite.
-- ════════════════════════════════════════════════════════════

-- ── otp_requests ──────────────────────────────────────────────
-- The otp_hash column already exists (VARCHAR 255).
-- Existing rows will have plain-text values — they expire naturally
-- within 5–15 minutes, so no backfill is required.

-- Ensure attempt_count column exists (was in original migration but verify).
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'otp_requests' AND column_name = 'attempt_count'
    ) THEN
        ALTER TABLE otp_requests ADD COLUMN attempt_count INTEGER NOT NULL DEFAULT 0;
    END IF;
END $$;

-- Add EMAIL_VERIFY to the OTP purpose constraint (for DBs that already ran migration 001).
DO $$
BEGIN
    ALTER TABLE otp_requests DROP CONSTRAINT IF EXISTS chk_otp_purpose;
    ALTER TABLE otp_requests ADD CONSTRAINT chk_otp_purpose CHECK (purpose IN (
        'LOGIN','REGISTER','COD_VERIFY','WITHDRAWAL_VERIFY','PASSWORD_RESET','EMAIL_VERIFY'
    ));
EXCEPTION WHEN others THEN NULL;
END $$;

-- Add max_attempts enforcement check (lockout after 10 wrong tries).
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE table_name = 'otp_requests' AND constraint_name = 'chk_otp_max_attempts'
    ) THEN
        ALTER TABLE otp_requests
            ADD CONSTRAINT chk_otp_max_attempts
            CHECK (attempt_count <= 10);
    END IF;
END $$;

-- ── sessions ──────────────────────────────────────────────────
-- The refresh_token_hash column already exists.
-- Existing plain-text tokens are revoked below so the system
-- starts clean with the new hashed scheme.

-- Revoke all existing sessions — they used plain-text tokens.
-- Users must log in again after this migration.
UPDATE sessions
SET    is_revoked = TRUE
WHERE  is_revoked = FALSE;

-- Add last_used_at column if not present (tracks token rotation activity).
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'sessions' AND column_name = 'last_used_at'
    ) THEN
        ALTER TABLE sessions ADD COLUMN last_used_at TIMESTAMPTZ;
    END IF;
END $$;

-- Partial index on active sessions for faster token lookups.
CREATE INDEX IF NOT EXISTS idx_sessions_active_token
    ON sessions(refresh_token_hash)
    WHERE is_revoked = FALSE AND expires_at > now();

-- ── users ─────────────────────────────────────────────────────
-- Set email_verified = FALSE for any user registered without email verification.
-- This ensures that the new registration flow (email_verified=FALSE by default)
-- is consistent with historical records.
-- NOTE: If you have already verified emails via another mechanism, comment this out.
-- UPDATE users SET email_verified = FALSE WHERE email IS NOT NULL AND email_verified = TRUE;
