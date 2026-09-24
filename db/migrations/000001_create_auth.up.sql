-- ════════════════════════════════════════════════════════════
-- Migration 0001: Identity & Auth Domain
-- ════════════════════════════════════════════════════════════

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ── users ────────────────────────────────────────────────────
CREATE TABLE users (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone                 VARCHAR(15)  UNIQUE,
    email                 VARCHAR(255) UNIQUE,
    password_hash         VARCHAR(255),
    full_name             VARCHAR(255) NOT NULL DEFAULT '',
    profile_picture_url   TEXT,
    role                  VARCHAR(30)  NOT NULL DEFAULT 'CUSTOMER',
    status                VARCHAR(30)  NOT NULL DEFAULT 'ACTIVE',
    email_verified        BOOLEAN      NOT NULL DEFAULT FALSE,
    phone_verified        BOOLEAN      NOT NULL DEFAULT FALSE,
    google_id             VARCHAR(255) UNIQUE,
    preferred_language    VARCHAR(10)  NOT NULL DEFAULT 'bn',
    date_of_birth         DATE,
    gender                VARCHAR(10),
    referral_code         VARCHAR(20)  UNIQUE,
    referred_by_id        UUID REFERENCES users(id) ON DELETE SET NULL,
    last_login_at         TIMESTAMPTZ,
    login_attempt_count   INTEGER      NOT NULL DEFAULT 0,
    locked_until          TIMESTAMPTZ,
    created_at            TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT chk_users_role CHECK (role IN (
        'CUSTOMER','SELLER','RESELLER','AFFILIATE',
        'ADMIN_OPS','ADMIN_FINANCE','SUPER_ADMIN'
    )),
    CONSTRAINT chk_users_status CHECK (status IN (
        'ACTIVE','BLOCKED','PENDING_VERIFICATION','SUSPENDED'
    )),
    CONSTRAINT chk_users_phone_or_email CHECK (
        phone IS NOT NULL OR email IS NOT NULL
    )
);

CREATE INDEX idx_users_phone  ON users(phone)  WHERE phone IS NOT NULL;
CREATE INDEX idx_users_email  ON users(email)  WHERE email IS NOT NULL;
CREATE INDEX idx_users_role   ON users(role);
CREATE INDEX idx_users_status ON users(status);
CREATE INDEX idx_users_google ON users(google_id) WHERE google_id IS NOT NULL;

-- ── otp_requests ─────────────────────────────────────────────
CREATE TABLE otp_requests (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    phone         VARCHAR(15)  NOT NULL,
    otp_hash      VARCHAR(255) NOT NULL,
    purpose       VARCHAR(50)  NOT NULL DEFAULT 'LOGIN',
    metadata      JSONB        NOT NULL DEFAULT '{}',
    expires_at    TIMESTAMPTZ  NOT NULL,
    used          BOOLEAN      NOT NULL DEFAULT FALSE,
    attempt_count INTEGER      NOT NULL DEFAULT 0,
    ip_address    INET,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT chk_otp_purpose CHECK (purpose IN (
        'LOGIN','REGISTER','COD_VERIFY','WITHDRAWAL_VERIFY','PASSWORD_RESET','EMAIL_VERIFY'
    ))
);

CREATE INDEX idx_otp_phone_used_exp ON otp_requests(phone, used, expires_at);
CREATE INDEX idx_otp_created        ON otp_requests(created_at);

-- ── sessions ─────────────────────────────────────────────────
CREATE TABLE sessions (
    id                  UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token_hash  VARCHAR(255) NOT NULL UNIQUE,
    ip_address          INET,
    user_agent          TEXT,
    device_name         VARCHAR(255),
    is_revoked          BOOLEAN      NOT NULL DEFAULT FALSE,
    expires_at          TIMESTAMPTZ  NOT NULL,
    last_used_at        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_sessions_user    ON sessions(user_id, is_revoked);
CREATE INDEX idx_sessions_token   ON sessions(refresh_token_hash);
CREATE INDEX idx_sessions_expires ON sessions(expires_at) WHERE is_revoked = FALSE;

-- ── oauth_providers ──────────────────────────────────────────
CREATE TABLE oauth_providers (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider      VARCHAR(20)  NOT NULL,
    provider_id   VARCHAR(255) NOT NULL,
    access_token  TEXT,
    refresh_token TEXT,
    expires_at    TIMESTAMPTZ,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),

    UNIQUE(provider, provider_id)
);

-- ── addresses ────────────────────────────────────────────────
CREATE TABLE addresses (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    label            VARCHAR(50)  NOT NULL DEFAULT 'Home',
    recipient_name   VARCHAR(255) NOT NULL,
    recipient_phone  VARCHAR(15)  NOT NULL,
    address_line1    TEXT         NOT NULL,
    address_line2    TEXT,
    city             VARCHAR(100) NOT NULL,
    district         VARCHAR(100) NOT NULL,
    upazila          VARCHAR(100),
    zip_code         VARCHAR(10),
    is_default       BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_addresses_user ON addresses(user_id);

-- ── audit_logs ───────────────────────────────────────────────
CREATE TABLE audit_logs (
    id          UUID         DEFAULT gen_random_uuid(),
    actor_id    UUID         REFERENCES users(id) ON DELETE SET NULL,
    actor_role  VARCHAR(30),
    action      VARCHAR(100) NOT NULL,
    entity_type VARCHAR(100) NOT NULL,
    entity_id   UUID,
    old_value   JSONB,
    new_value   JSONB,
    reason      TEXT,
    ip_address  INET,
    user_agent  TEXT,
    timestamp   TIMESTAMPTZ  NOT NULL DEFAULT now(),

    PRIMARY KEY (id, timestamp)
) PARTITION BY RANGE (timestamp);

CREATE TABLE audit_logs_2024 PARTITION OF audit_logs
    FOR VALUES FROM ('2024-01-01') TO ('2025-01-01');
CREATE TABLE audit_logs_2025 PARTITION OF audit_logs
    FOR VALUES FROM ('2025-01-01') TO ('2026-01-01');
CREATE TABLE audit_logs_2026 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-01-01') TO ('2027-01-01');

CREATE INDEX idx_audit_actor     ON audit_logs(actor_id);
CREATE INDEX idx_audit_entity    ON audit_logs(entity_type, entity_id);
CREATE INDEX idx_audit_timestamp ON audit_logs(timestamp DESC);

-- Prevent UPDATE/DELETE on audit_logs (immutability)
CREATE OR REPLACE FUNCTION prevent_audit_log_modification()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'audit_logs are immutable — no modifications allowed';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_audit_logs_no_update
    BEFORE UPDATE ON audit_logs
    FOR EACH ROW EXECUTE FUNCTION prevent_audit_log_modification();

CREATE TRIGGER trg_audit_logs_no_delete
    BEFORE DELETE ON audit_logs
    FOR EACH ROW EXECUTE FUNCTION prevent_audit_log_modification();

-- Auto-update updated_at trigger function (reused across all tables)
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_sessions_updated_at
    BEFORE UPDATE ON sessions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
