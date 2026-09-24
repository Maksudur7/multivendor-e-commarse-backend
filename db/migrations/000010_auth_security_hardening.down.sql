-- ════════════════════════════════════════════════════════════
-- Migration 0010 DOWN: Revert Auth Security Hardening
-- ════════════════════════════════════════════════════════════

-- Drop the partial active-token index.
DROP INDEX IF EXISTS idx_sessions_active_token;

-- Remove the attempt_count constraint (revert to no limit).
ALTER TABLE otp_requests
    DROP CONSTRAINT IF EXISTS chk_otp_max_attempts;

-- NOTE: Revoked sessions and dropped columns cannot be fully restored.
-- A manual restore from backup is required for full rollback.
