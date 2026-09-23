-- ════════════════════════════════════════════════════════════
-- Migration 0008: Reviews, Notifications, Fraud, Search Logs
-- ════════════════════════════════════════════════════════════

-- ── reviews ───────────────────────────────────────────────────
CREATE TABLE reviews (
    id                    UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id            UUID        NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    variant_id            UUID        REFERENCES product_variants(id) ON DELETE SET NULL,
    order_item_id         UUID        NOT NULL UNIQUE REFERENCES order_items(id) ON DELETE RESTRICT,
    reviewer_id           UUID        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    rating                SMALLINT    NOT NULL CHECK (rating BETWEEN 1 AND 5),
    title                 VARCHAR(100),
    body                  TEXT,
    status                VARCHAR(30) NOT NULL DEFAULT 'PENDING_MODERATION',
    rejection_reason      TEXT,
    moderated_by          UUID        REFERENCES users(id) ON DELETE SET NULL,
    moderated_at          TIMESTAMPTZ,
    helpful_count         INTEGER     NOT NULL DEFAULT 0,
    not_helpful_count     INTEGER     NOT NULL DEFAULT 0,
    is_verified_purchase  BOOLEAN     NOT NULL DEFAULT TRUE,
    seller_response       TEXT,
    seller_responded_at   TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT chk_review_status CHECK (status IN (
        'PENDING_MODERATION','APPROVED','REJECTED','HIDDEN'
    ))
);

CREATE INDEX idx_reviews_product    ON reviews(product_id, status);
CREATE INDEX idx_reviews_reviewer   ON reviews(reviewer_id);
CREATE INDEX idx_reviews_order_item ON reviews(order_item_id);
CREATE INDEX idx_reviews_moderation ON reviews(status, created_at) WHERE status = 'PENDING_MODERATION';

CREATE TRIGGER trg_reviews_updated_at
    BEFORE UPDATE ON reviews
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ── review_media ──────────────────────────────────────────────
CREATE TABLE review_media (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    review_id   UUID        NOT NULL REFERENCES reviews(id) ON DELETE CASCADE,
    url         TEXT        NOT NULL,
    thumbnail_url TEXT,
    media_type  VARCHAR(10) NOT NULL DEFAULT 'IMAGE',
    sort_order  INTEGER     NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_review_media_review ON review_media(review_id);

-- ── notification_templates ────────────────────────────────────
CREATE TABLE notification_templates (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type   VARCHAR(100) NOT NULL,
    channel      VARCHAR(20)  NOT NULL,
    language     VARCHAR(10)  NOT NULL DEFAULT 'bn',
    template_body TEXT        NOT NULL,
    subject      VARCHAR(255),
    is_active    BOOLEAN      NOT NULL DEFAULT TRUE,
    created_by   UUID         REFERENCES users(id) ON DELETE SET NULL,
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),

    UNIQUE(event_type, channel, language),
    CONSTRAINT chk_notif_channel CHECK (channel IN ('SMS','WHATSAPP','EMAIL','PUSH'))
);

CREATE TRIGGER trg_notif_templates_updated_at
    BEFORE UPDATE ON notification_templates
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ── device_tokens ─────────────────────────────────────────────
CREATE TABLE device_tokens (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    fcm_token   TEXT        NOT NULL,
    platform    VARCHAR(10) NOT NULL DEFAULT 'ANDROID',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen   TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE(user_id, fcm_token),
    CONSTRAINT chk_device_platform CHECK (platform IN ('ANDROID','IOS','WEB'))
);

CREATE INDEX idx_device_tokens_user ON device_tokens(user_id);

-- ── notification_logs ─────────────────────────────────────────
CREATE TABLE notification_logs (
    id               UUID        DEFAULT gen_random_uuid(),
    user_id          UUID        REFERENCES users(id) ON DELETE SET NULL,
    event_type       VARCHAR(100),
    channel          VARCHAR(20) NOT NULL,
    recipient        VARCHAR(255) NOT NULL,
    template_id      UUID        REFERENCES notification_templates(id) ON DELETE SET NULL,
    rendered_content TEXT,
    status           VARCHAR(30) NOT NULL DEFAULT 'QUEUED',
    gateway_response JSONB       NOT NULL DEFAULT '{}',
    error_message    TEXT,
    attempt_count    INTEGER     NOT NULL DEFAULT 1,
    next_retry_at    TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_at          TIMESTAMPTZ,

    PRIMARY KEY (id, created_at),
    CONSTRAINT chk_notif_status CHECK (status IN (
        'QUEUED','SENT','DELIVERED','FAILED'
    ))
) PARTITION BY RANGE (created_at);

CREATE TABLE notification_logs_2024 PARTITION OF notification_logs
    FOR VALUES FROM ('2024-01-01') TO ('2025-01-01');
CREATE TABLE notification_logs_2025 PARTITION OF notification_logs
    FOR VALUES FROM ('2025-01-01') TO ('2026-01-01');
CREATE TABLE notification_logs_2026 PARTITION OF notification_logs
    FOR VALUES FROM ('2026-01-01') TO ('2027-01-01');

CREATE INDEX idx_notif_logs_user    ON notification_logs(user_id, created_at DESC);
CREATE INDEX idx_notif_logs_retry   ON notification_logs(status, next_retry_at)
    WHERE status = 'FAILED';

-- ── buyer_risk_profiles ───────────────────────────────────────
CREATE TABLE buyer_risk_profiles (
    id                    UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id               UUID         NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    risk_score            DECIMAL(5,2) NOT NULL DEFAULT 0 CHECK (risk_score BETWEEN 0 AND 100),
    risk_level            VARCHAR(20)  NOT NULL DEFAULT 'LOW',
    total_orders          INTEGER      NOT NULL DEFAULT 0,
    total_delivered       INTEGER      NOT NULL DEFAULT 0,
    total_returns         INTEGER      NOT NULL DEFAULT 0,
    total_cod_refused     INTEGER      NOT NULL DEFAULT 0,
    total_cod_orders      INTEGER      NOT NULL DEFAULT 0,
    return_rate           DECIMAL(5,2) NOT NULL DEFAULT 0,
    cod_refusal_rate      DECIMAL(5,2) NOT NULL DEFAULT 0,
    score_components      JSONB        NOT NULL DEFAULT '{}',
    last_recalculated_at  TIMESTAMPTZ,
    manually_reviewed     BOOLEAN      NOT NULL DEFAULT FALSE,
    admin_note            TEXT,
    reviewed_by           UUID         REFERENCES users(id) ON DELETE SET NULL,
    created_at            TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT chk_risk_level CHECK (risk_level IN ('LOW','MEDIUM','HIGH','VERY_HIGH'))
);

CREATE INDEX idx_risk_profiles_user  ON buyer_risk_profiles(user_id);
CREATE INDEX idx_risk_profiles_score ON buyer_risk_profiles(risk_score DESC);

CREATE TRIGGER trg_risk_profiles_updated_at
    BEFORE UPDATE ON buyer_risk_profiles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ── blacklisted_phones ────────────────────────────────────────
CREATE TABLE blacklisted_phones (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    phone           VARCHAR(15) UNIQUE NOT NULL,
    reason          VARCHAR(50) NOT NULL,
    note            TEXT,
    is_active       BOOLEAN     NOT NULL DEFAULT TRUE,
    blacklisted_by  UUID        REFERENCES users(id) ON DELETE SET NULL,
    expires_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT chk_blacklist_reason CHECK (reason IN (
        'FRAUD','ABUSIVE','FAKE_ORDERS','EXCESSIVE_RETURNS','OTHER'
    ))
);

CREATE INDEX idx_blacklist_phone  ON blacklisted_phones(phone, is_active);

-- ── ip_blocks ─────────────────────────────────────────────────
CREATE TABLE ip_blocks (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    ip_address    INET        NOT NULL,
    reason        VARCHAR(100),
    blocked_until TIMESTAMPTZ NOT NULL,
    block_count   INTEGER     NOT NULL DEFAULT 1,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_ip_blocks_addr ON ip_blocks(ip_address, blocked_until);

-- ── search_logs ───────────────────────────────────────────────
CREATE TABLE search_logs (
    id                 UUID        DEFAULT gen_random_uuid(),
    query              VARCHAR(500) NOT NULL,
    user_id            UUID        REFERENCES users(id) ON DELETE SET NULL,
    result_count       INTEGER     NOT NULL DEFAULT 0,
    filters_used       JSONB       NOT NULL DEFAULT '{}',
    clicked_product_id UUID        REFERENCES products(id) ON DELETE SET NULL,
    position_clicked   INTEGER,
    session_id         VARCHAR(255),
    searched_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (id, searched_at)
) PARTITION BY RANGE (searched_at);

CREATE TABLE search_logs_2024 PARTITION OF search_logs
    FOR VALUES FROM ('2024-01-01') TO ('2025-01-01');
CREATE TABLE search_logs_2025 PARTITION OF search_logs
    FOR VALUES FROM ('2025-01-01') TO ('2026-01-01');
CREATE TABLE search_logs_2026 PARTITION OF search_logs
    FOR VALUES FROM ('2026-01-01') TO ('2027-01-01');

CREATE INDEX idx_search_logs_query ON search_logs(searched_at DESC);

-- ── wishlist ──────────────────────────────────────────────────
CREATE TABLE wishlists (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    product_id UUID        NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    variant_id UUID        REFERENCES product_variants(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE(user_id, product_id)
);

CREATE INDEX idx_wishlist_user    ON wishlists(user_id);
CREATE INDEX idx_wishlist_product ON wishlists(product_id);

-- ── platform_settings ──────────────────────────────────────────
CREATE TABLE platform_settings (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    key         VARCHAR(100) UNIQUE NOT NULL,
    value       TEXT         NOT NULL,
    description TEXT,
    is_sensitive BOOLEAN     NOT NULL DEFAULT FALSE,
    updated_by  UUID         REFERENCES users(id) ON DELETE SET NULL,
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- Insert default settings
INSERT INTO platform_settings (key, value, description) VALUES
    ('default_commission_rate', '10.0',  'Default platform commission %'),
    ('cod_limit_default',       '15000', 'Default COD limit BDT'),
    ('cod_limit_new_user',      '5000',  'COD limit for new users BDT'),
    ('return_window_days',      '7',     'Default return window days'),
    ('escrow_release_days',     '7',     'Days after delivery before escrow releases'),
    ('min_seller_payout',       '500',   'Minimum seller withdrawal BDT'),
    ('min_reseller_payout',     '200',   'Minimum reseller withdrawal BDT'),
    ('min_affiliate_payout',    '500',   'Minimum affiliate withdrawal BDT'),
    ('fraud_block_score',       '81',    'Auto-block COD above this risk score'),
    ('fraud_review_score',      '61',    'Manual review above this risk score');

-- ── report_jobs ───────────────────────────────────────────────
CREATE TABLE report_jobs (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    report_type  VARCHAR(100) NOT NULL,
    parameters   JSONB       NOT NULL DEFAULT '{}',
    status       VARCHAR(30) NOT NULL DEFAULT 'QUEUED',
    file_url     TEXT,
    error_msg    TEXT,
    requested_by UUID        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,

    CONSTRAINT chk_report_status CHECK (status IN ('QUEUED','PROCESSING','COMPLETED','FAILED'))
);

CREATE INDEX idx_report_jobs_user ON report_jobs(requested_by, created_at DESC);
