-- ════════════════════════════════════════════════════════════
-- Migration 0007: Affiliate & Reseller Portals
-- ════════════════════════════════════════════════════════════

-- ── resellers ─────────────────────────────────────────────────
CREATE TABLE resellers (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID         NOT NULL UNIQUE REFERENCES users(id) ON DELETE RESTRICT,
    status           VARCHAR(30)  NOT NULL DEFAULT 'PENDING',
    business_name    VARCHAR(255),
    nid_number       VARCHAR(255),
    nid_front_url    TEXT,
    nid_back_url     TEXT,
    facebook_page_url TEXT,
    payout_method    VARCHAR(20),
    payout_account   JSONB        NOT NULL DEFAULT '{}',
    rejection_reason TEXT,
    approved_by      UUID         REFERENCES users(id) ON DELETE SET NULL,
    approved_at      TIMESTAMPTZ,
    reseller_tier    VARCHAR(20)  NOT NULL DEFAULT 'STANDARD',
    total_orders     INTEGER      NOT NULL DEFAULT 0,
    total_earned     DECIMAL(14,2) NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT chk_reseller_status CHECK (status IN (
        'PENDING','ACTIVE','SUSPENDED','REJECTED'
    )),
    CONSTRAINT chk_reseller_tier CHECK (reseller_tier IN (
        'STANDARD','SILVER','GOLD','PLATINUM'
    ))
);

CREATE INDEX idx_resellers_user   ON resellers(user_id);
CREATE INDEX idx_resellers_status ON resellers(status);
CREATE INDEX idx_resellers_tier   ON resellers(reseller_tier);

CREATE TRIGGER trg_resellers_updated_at
    BEFORE UPDATE ON resellers
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ── affiliate_applications ────────────────────────────────────
CREATE TABLE affiliate_applications (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    status           VARCHAR(30) NOT NULL DEFAULT 'PENDING',
    website_url      TEXT,
    social_links     JSONB       NOT NULL DEFAULT '{}',
    traffic_estimate VARCHAR(50),
    content_niche    VARCHAR(100),
    promotion_method TEXT,
    rejection_reason TEXT,
    reviewed_by      UUID        REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at      TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT chk_affiliate_app_status CHECK (status IN (
        'PENDING','APPROVED','REJECTED'
    ))
);

CREATE INDEX idx_aff_apps_user   ON affiliate_applications(user_id);
CREATE INDEX idx_aff_apps_status ON affiliate_applications(status);

-- ── affiliates ────────────────────────────────────────────────
CREATE TABLE affiliates (
    id                      UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                 UUID         NOT NULL UNIQUE REFERENCES users(id) ON DELETE RESTRICT,
    status                  VARCHAR(30)  NOT NULL DEFAULT 'ACTIVE',
    default_commission_rate DECIMAL(5,2) NOT NULL DEFAULT 5.00,
    custom_rates            JSONB        NOT NULL DEFAULT '{}',
    referral_code           VARCHAR(20)  UNIQUE NOT NULL,
    total_clicks            BIGINT       NOT NULL DEFAULT 0,
    total_conversions       BIGINT       NOT NULL DEFAULT 0,
    total_earned            DECIMAL(14,2) NOT NULL DEFAULT 0,
    joined_at               TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT chk_affiliate_status CHECK (status IN (
        'ACTIVE','SUSPENDED','TERMINATED'
    ))
);

CREATE INDEX idx_affiliates_user ON affiliates(user_id);
CREATE INDEX idx_affiliates_code ON affiliates(referral_code);

-- ── affiliate_links ───────────────────────────────────────────
CREATE TABLE affiliate_links (
    id                UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    affiliate_id      UUID         NOT NULL REFERENCES affiliates(id) ON DELETE RESTRICT,
    target_type       VARCHAR(30)  NOT NULL,
    target_id         UUID,
    full_url          TEXT         NOT NULL,
    short_code        VARCHAR(30)  UNIQUE NOT NULL,
    custom_alias      VARCHAR(50)  UNIQUE,
    utm_source        VARCHAR(100),
    utm_medium        VARCHAR(100),
    utm_campaign      VARCHAR(100),
    label             VARCHAR(255),
    total_clicks      BIGINT       NOT NULL DEFAULT 0,
    unique_clicks     BIGINT       NOT NULL DEFAULT 0,
    total_conversions BIGINT       NOT NULL DEFAULT 0,
    is_active         BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT chk_link_target CHECK (target_type IN (
        'PRODUCT','CATEGORY','HOMEPAGE','SEARCH','CUSTOM'
    ))
);

CREATE INDEX idx_aff_links_affiliate ON affiliate_links(affiliate_id);
CREATE INDEX idx_aff_links_short     ON affiliate_links(short_code);
CREATE INDEX idx_aff_links_active    ON affiliate_links(is_active, affiliate_id);

-- ── link_clicks ───────────────────────────────────────────────
CREATE TABLE link_clicks (
    id           UUID         DEFAULT gen_random_uuid(),
    link_id      UUID         NOT NULL REFERENCES affiliate_links(id) ON DELETE RESTRICT,
    affiliate_id UUID         NOT NULL REFERENCES affiliates(id) ON DELETE RESTRICT,
    fingerprint  VARCHAR(255),
    ip_hash      VARCHAR(255),
    user_agent   TEXT,
    referer      TEXT,
    is_unique    BOOLEAN      NOT NULL DEFAULT TRUE,
    user_id      UUID         REFERENCES users(id) ON DELETE SET NULL,
    clicked_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),

    PRIMARY KEY (id, clicked_at)
) PARTITION BY RANGE (clicked_at);

CREATE TABLE link_clicks_2024 PARTITION OF link_clicks
    FOR VALUES FROM ('2024-01-01') TO ('2025-01-01');
CREATE TABLE link_clicks_2025 PARTITION OF link_clicks
    FOR VALUES FROM ('2025-01-01') TO ('2026-01-01');
CREATE TABLE link_clicks_2026 PARTITION OF link_clicks
    FOR VALUES FROM ('2026-01-01') TO ('2027-01-01');

CREATE INDEX idx_clicks_link      ON link_clicks(link_id, clicked_at DESC);
CREATE INDEX idx_clicks_affiliate ON link_clicks(affiliate_id, clicked_at DESC);

-- ── affiliate_conversions ─────────────────────────────────────
CREATE TABLE affiliate_conversions (
    id                  UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    affiliate_id        UUID          NOT NULL REFERENCES affiliates(id) ON DELETE RESTRICT,
    link_id             UUID          NOT NULL REFERENCES affiliate_links(id) ON DELETE RESTRICT,
    order_id            UUID          NOT NULL UNIQUE REFERENCES orders(id) ON DELETE RESTRICT,
    click_id            UUID,
    commission_amount   DECIMAL(12,2) NOT NULL,
    commission_rate     DECIMAL(5,2)  NOT NULL,
    status              VARCHAR(30)   NOT NULL DEFAULT 'PENDING',
    attribution_window  INTEGER       NOT NULL DEFAULT 30,
    converted_at        TIMESTAMPTZ   NOT NULL DEFAULT now(),
    confirmed_at        TIMESTAMPTZ,
    paid_at             TIMESTAMPTZ,

    CONSTRAINT chk_conversion_status CHECK (status IN (
        'PENDING','CONFIRMED','CANCELLED','PAID'
    ))
);

CREATE INDEX idx_conversions_affiliate ON affiliate_conversions(affiliate_id, status);
CREATE INDEX idx_conversions_order     ON affiliate_conversions(order_id);
CREATE INDEX idx_conversions_link      ON affiliate_conversions(link_id);
