-- ════════════════════════════════════════════════════════════
-- Migration 0001 DOWN: Drop Identity & Auth Domain
-- ════════════════════════════════════════════════════════════

DROP TRIGGER IF EXISTS trg_users_updated_at ON users;
DROP TRIGGER IF EXISTS trg_sessions_updated_at ON sessions;
DROP TRIGGER IF EXISTS trg_audit_logs_no_update ON audit_logs;
DROP TRIGGER IF EXISTS trg_audit_logs_no_delete ON audit_logs;
DROP FUNCTION IF EXISTS prevent_audit_log_modification();
DROP FUNCTION IF EXISTS set_updated_at();
DROP TABLE IF EXISTS audit_logs CASCADE;
DROP TABLE IF EXISTS addresses CASCADE;
DROP TABLE IF EXISTS oauth_providers CASCADE;
DROP TABLE IF EXISTS sessions CASCADE;
DROP TABLE IF EXISTS otp_requests CASCADE;
DROP TABLE IF EXISTS users CASCADE;
DROP EXTENSION IF EXISTS pgcrypto;
