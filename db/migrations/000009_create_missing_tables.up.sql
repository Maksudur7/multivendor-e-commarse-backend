-- ════════════════════════════════════════════════════════════
-- Migration 0009: Align Handler Tables with Actual DB Schema
-- Uses CREATE TABLE IF NOT EXISTS and ALTER TABLE ADD COLUMN IF NOT EXISTS
-- ════════════════════════════════════════════════════════════

-- ── product_reviews (our handlers use this, not the "reviews" table) ──
CREATE TABLE IF NOT EXISTS product_reviews (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id           UUID        NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    user_id              UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rating               SMALLINT    NOT NULL CHECK (rating BETWEEN 1 AND 5),
    title                VARCHAR(200),
    comment              TEXT,
    is_verified_purchase BOOLEAN     NOT NULL DEFAULT TRUE,
    helpful_count        INTEGER     NOT NULL DEFAULT 0,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_product_reviews_product ON product_reviews(product_id);
CREATE INDEX IF NOT EXISTS idx_product_reviews_user    ON product_reviews(user_id);

-- ── notifications table ────────────────────────────────────────
CREATE TABLE IF NOT EXISTS notifications (
    id         UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title      VARCHAR(255) NOT NULL,
    body       TEXT         NOT NULL,
    type       VARCHAR(50)  NOT NULL DEFAULT 'INFO',
    is_read    BOOLEAN      NOT NULL DEFAULT FALSE,
    metadata   JSONB        NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_notifications_user   ON notifications(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_unread ON notifications(user_id, is_read) WHERE is_read = FALSE;

-- ── Align wallets: our handlers use user_id, pending_escrow, total_withdrawn ──
-- The existing schema uses owner_id and pending_balance. Add aliases.
DO $$
BEGIN
    -- Add user_id alias column if not exists (wallets.owner_id -> user_id)
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='wallets' AND column_name='user_id') THEN
        ALTER TABLE wallets ADD COLUMN user_id UUID REFERENCES users(id) ON DELETE RESTRICT;
        UPDATE wallets SET user_id = owner_id WHERE user_id IS NULL;
        CREATE UNIQUE INDEX IF NOT EXISTS idx_wallets_user_id ON wallets(user_id);
    END IF;

    -- Add pending_escrow alias for pending_balance
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='wallets' AND column_name='pending_escrow') THEN
        ALTER TABLE wallets ADD COLUMN pending_escrow DECIMAL(14,2) NOT NULL DEFAULT 0;
        UPDATE wallets SET pending_escrow = pending_balance WHERE pending_escrow = 0;
    END IF;

    -- Add total_withdrawn column
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='wallets' AND column_name='total_withdrawn') THEN
        ALTER TABLE wallets ADD COLUMN total_withdrawn DECIMAL(14,2) NOT NULL DEFAULT 0;
    END IF;
END;
$$;

-- ── wallet_transactions (our ledger handler references this) ───
CREATE TABLE IF NOT EXISTS wallet_transactions (
    id               UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_id        UUID          NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
    amount           DECIMAL(12,2) NOT NULL,
    transaction_type VARCHAR(30)   NOT NULL DEFAULT 'CREDIT',
    description      TEXT,
    reference_id     UUID,
    created_at       TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_wallet_txs_wallet ON wallet_transactions(wallet_id, created_at DESC);

-- ── payout_requests: add transaction_proof_ref if missing ─────
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='payout_requests' AND column_name='transaction_proof_ref') THEN
        ALTER TABLE payout_requests ADD COLUMN transaction_proof_ref TEXT;
    END IF;
END;
$$;

-- ── master_orders: ensure vendor_id column exists ──────────────
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='master_orders' AND column_name='vendor_id') THEN
        ALTER TABLE master_orders ADD COLUMN vendor_id UUID REFERENCES vendors(id) ON DELETE SET NULL;
    END IF;
END;
$$;

-- ── refunds table ──────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS refunds (
    id          UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID          NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount      DECIMAL(12,2) NOT NULL,
    reason      TEXT,
    status      VARCHAR(30)   NOT NULL DEFAULT 'PROCESSING',
    created_at  TIMESTAMPTZ   NOT NULL DEFAULT now(),
    CONSTRAINT chk_refund_status CHECK (status IN ('PROCESSING','APPROVED','REJECTED','COMPLETED'))
);

CREATE INDEX IF NOT EXISTS idx_refunds_user ON refunds(user_id);

-- ── china_sourcing_batches ─────────────────────────────────────
CREATE TABLE IF NOT EXISTS china_sourcing_batches (
    id               UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_code       VARCHAR(50)   UNIQUE NOT NULL,
    user_id          UUID          REFERENCES users(id) ON DELETE SET NULL,
    status           VARCHAR(30)   NOT NULL DEFAULT 'SOURCING',
    cny_cost         DECIMAL(12,2),
    freight_cost     DECIMAL(12,2),
    customs_duty_pct DECIMAL(5,2),
    landed_cost_bdt  DECIMAL(14,2),
    customs_info     JSONB         NOT NULL DEFAULT '{}',
    created_at       TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ   NOT NULL DEFAULT now(),
    CONSTRAINT chk_batch_status CHECK (status IN (
        'SOURCING','ORDERED','SHIPPED','IN_TRANSIT_AIR','IN_TRANSIT_SEA',
        'CUSTOMS','WAREHOUSE','DELIVERED','COMPLETED','CANCELLED'
    ))
);

CREATE INDEX IF NOT EXISTS idx_china_batches_user   ON china_sourcing_batches(user_id);
CREATE INDEX IF NOT EXISTS idx_china_batches_status ON china_sourcing_batches(status);

-- ── china_sourcing_batch_items ─────────────────────────────────
CREATE TABLE IF NOT EXISTS china_sourcing_batch_items (
    id              UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id        UUID          NOT NULL REFERENCES china_sourcing_batches(id) ON DELETE CASCADE,
    product_name    VARCHAR(255),
    sku_code        VARCHAR(100),
    quantity        INTEGER       NOT NULL DEFAULT 1,
    unit_cost_cny   DECIMAL(10,2),
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_china_batch_items ON china_sourcing_batch_items(batch_id);

-- ── flash_sales ────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS flash_sales (
    id           UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    title        VARCHAR(255)  NOT NULL,
    discount_pct DECIMAL(5,2)  NOT NULL DEFAULT 0,
    starts_at    TIMESTAMPTZ   NOT NULL DEFAULT now(),
    ends_at      TIMESTAMPTZ   NOT NULL,
    is_active    BOOLEAN       NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_flash_sales_active ON flash_sales(is_active, ends_at);

-- ── affiliate_profiles ─────────────────────────────────────────
CREATE TABLE IF NOT EXISTS affiliate_profiles (
    id            UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID          NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    status        VARCHAR(30)   NOT NULL DEFAULT 'PENDING',
    referral_code VARCHAR(30)   UNIQUE NOT NULL,
    total_clicks  BIGINT        NOT NULL DEFAULT 0,
    total_earned  DECIMAL(14,2) NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ   NOT NULL DEFAULT now(),
    CONSTRAINT chk_aff_status CHECK (status IN ('PENDING','APPROVED','SUSPENDED','REJECTED'))
);

CREATE INDEX IF NOT EXISTS idx_aff_profiles_user ON affiliate_profiles(user_id);
CREATE INDEX IF NOT EXISTS idx_aff_profiles_code ON affiliate_profiles(referral_code);

-- ── affiliate_links_simple ─────────────────────────────────────
CREATE TABLE IF NOT EXISTS affiliate_links_simple (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    affiliate_id UUID        NOT NULL REFERENCES affiliate_profiles(id) ON DELETE CASCADE,
    product_id   UUID        REFERENCES products(id) ON DELETE SET NULL,
    short_code   VARCHAR(30) UNIQUE NOT NULL,
    full_url     TEXT        NOT NULL,
    total_clicks BIGINT      NOT NULL DEFAULT 0,
    is_active    BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_aff_links_simple ON affiliate_links_simple(affiliate_id);

-- ── affiliate_withdrawals ─────────────────────────────────────
CREATE TABLE IF NOT EXISTS affiliate_withdrawals (
    id           UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    affiliate_id UUID          NOT NULL REFERENCES affiliate_profiles(id) ON DELETE CASCADE,
    user_id      UUID          NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount       DECIMAL(12,2) NOT NULL,
    status       VARCHAR(30)   NOT NULL DEFAULT 'PENDING',
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT now(),
    CONSTRAINT chk_aff_withdrawal_status CHECK (status IN ('PENDING','APPROVED','REJECTED','PAID'))
);

-- ── report_jobs (for admin finance reports) ────────────────────
CREATE TABLE IF NOT EXISTS report_jobs (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    report_type  VARCHAR(100) NOT NULL,
    parameters   JSONB        NOT NULL DEFAULT '{}',
    status       VARCHAR(30)  NOT NULL DEFAULT 'QUEUED',
    file_url     TEXT,
    error_msg    TEXT,
    requested_by UUID         NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    CONSTRAINT chk_report_status CHECK (status IN ('QUEUED','PROCESSING','COMPLETED','FAILED'))
);

CREATE INDEX IF NOT EXISTS idx_report_jobs_user ON report_jobs(requested_by, created_at DESC);

-- ── platform_settings ──────────────────────────────────────────
CREATE TABLE IF NOT EXISTS platform_settings (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    key         VARCHAR(100) UNIQUE NOT NULL,
    value       TEXT         NOT NULL,
    description TEXT,
    is_sensitive BOOLEAN     NOT NULL DEFAULT FALSE,
    updated_by  UUID         REFERENCES users(id) ON DELETE SET NULL,
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- Seed default settings if not exist
INSERT INTO platform_settings (key, value, description) VALUES
    ('default_commission_rate', '10.0',  'Default platform commission %'),
    ('cod_limit_default',       '15000', 'Default COD limit BDT'),
    ('return_window_days',      '7',     'Default return window days'),
    ('escrow_release_days',     '7',     'Days after delivery before escrow releases'),
    ('min_seller_payout',       '500',   'Minimum seller withdrawal BDT'),
    ('maintenance_mode',        'false', 'Is platform in maintenance mode')
ON CONFLICT (key) DO NOTHING;

-- ── inventory_stock: ensure sku_id column exists ───────────────
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='inventory_stock' AND column_name='sku_id') THEN
        ALTER TABLE inventory_stock ADD COLUMN sku_id UUID REFERENCES product_skus(id) ON DELETE SET NULL;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='inventory_stock' AND column_name='warehouse_name') THEN
        ALTER TABLE inventory_stock ADD COLUMN warehouse_name VARCHAR(100) DEFAULT 'Main Hub Dhaka';
    END IF;
END;
$$;
