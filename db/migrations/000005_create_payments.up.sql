-- ════════════════════════════════════════════════════════════
-- Migration 0005: Payment & Escrow Domain
-- ════════════════════════════════════════════════════════════

-- ── payments ─────────────────────────────────────────────────
CREATE TABLE payments (
    id                  UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id            UUID          NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
    gateway             VARCHAR(30)   NOT NULL,
    amount              DECIMAL(12,2) NOT NULL CHECK (amount > 0),
    currency            VARCHAR(5)    NOT NULL DEFAULT 'BDT',
    status              VARCHAR(30)   NOT NULL DEFAULT 'INITIATED',
    gateway_txn_id      VARCHAR(255),
    gateway_session_id  VARCHAR(255),
    idempotency_key     VARCHAR(255)  UNIQUE NOT NULL,
    raw_response        JSONB         NOT NULL DEFAULT '{}',
    refund_amount       DECIMAL(12,2) NOT NULL DEFAULT 0,
    refunded_at         TIMESTAMPTZ,
    created_at          TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ   NOT NULL DEFAULT now(),

    CONSTRAINT chk_payment_gateway CHECK (gateway IN (
        'SSLCOMMERZ','BKASH','NAGAD','COD','WALLET'
    )),
    CONSTRAINT chk_payment_status CHECK (status IN (
        'INITIATED','PENDING','COMPLETED','FAILED',
        'REFUNDED','PARTIALLY_REFUNDED','CANCELLED'
    ))
);

CREATE INDEX idx_payments_order       ON payments(order_id);
CREATE INDEX idx_payments_gateway_txn ON payments(gateway_txn_id) WHERE gateway_txn_id IS NOT NULL;
CREATE INDEX idx_payments_idempotency ON payments(idempotency_key);
CREATE INDEX idx_payments_status      ON payments(status);

CREATE TRIGGER trg_payments_updated_at
    BEFORE UPDATE ON payments
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ── payment_events (idempotency log) ─────────────────────────
CREATE TABLE payment_events (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_id   UUID        NOT NULL REFERENCES payments(id) ON DELETE RESTRICT,
    event_type   VARCHAR(50) NOT NULL,
    gateway      VARCHAR(30) NOT NULL,
    event_id     VARCHAR(255) UNIQUE NOT NULL,
    raw_payload  JSONB       NOT NULL DEFAULT '{}',
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_payment_events_payment ON payment_events(payment_id);
CREATE INDEX idx_payment_events_event   ON payment_events(event_id);

-- ── wallets ───────────────────────────────────────────────────
CREATE TABLE wallets (
    id                UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id          UUID          NOT NULL UNIQUE REFERENCES users(id) ON DELETE RESTRICT,
    wallet_type       VARCHAR(20)   NOT NULL,
    available_balance DECIMAL(14,2) NOT NULL DEFAULT 0 CHECK (available_balance >= 0),
    pending_balance   DECIMAL(14,2) NOT NULL DEFAULT 0 CHECK (pending_balance >= 0),
    lifetime_earned   DECIMAL(14,2) NOT NULL DEFAULT 0,
    total_withdrawn   DECIMAL(14,2) NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ   NOT NULL DEFAULT now(),

    CONSTRAINT chk_wallet_type CHECK (wallet_type IN (
        'SELLER','RESELLER','AFFILIATE','CUSTOMER_CREDIT'
    ))
);

CREATE INDEX idx_wallets_owner ON wallets(owner_id);
CREATE INDEX idx_wallets_type  ON wallets(wallet_type);

CREATE TRIGGER trg_wallets_updated_at
    BEFORE UPDATE ON wallets
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ── wallet_transactions (immutable ledger) ────────────────────
CREATE TABLE wallet_transactions (
    id             UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_id      UUID          NOT NULL REFERENCES wallets(id) ON DELETE RESTRICT,
    type           VARCHAR(10)   NOT NULL,
    category       VARCHAR(50)   NOT NULL,
    amount         DECIMAL(12,2) NOT NULL CHECK (amount > 0),
    balance_after  DECIMAL(12,2) NOT NULL,
    reference_type VARCHAR(50),
    reference_id   UUID,
    description    TEXT,
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT now(),

    CONSTRAINT chk_wallet_tx_type CHECK (type IN ('CREDIT','DEBIT')),
    CONSTRAINT chk_wallet_tx_category CHECK (category IN (
        'ESCROW_RELEASE','COMMISSION_EARNED','REFUND_RECEIVED',
        'WITHDRAWAL','MANUAL_CREDIT','CLAWBACK','ORDER_PROFIT',
        'AFFILIATE_COMMISSION','PLATFORM_FEE'
    ))
) PARTITION BY RANGE (created_at);

CREATE TABLE wallet_transactions_2024 PARTITION OF wallet_transactions
    FOR VALUES FROM ('2024-01-01') TO ('2025-01-01');
CREATE TABLE wallet_transactions_2025 PARTITION OF wallet_transactions
    FOR VALUES FROM ('2025-01-01') TO ('2026-01-01');
CREATE TABLE wallet_transactions_2026 PARTITION OF wallet_transactions
    FOR VALUES FROM ('2026-01-01') TO ('2027-01-01');

CREATE INDEX idx_wallet_tx_wallet ON wallet_transactions(wallet_id, created_at DESC);
CREATE INDEX idx_wallet_tx_ref    ON wallet_transactions(reference_type, reference_id)
    WHERE reference_id IS NOT NULL;

-- Prevent modifications
CREATE TRIGGER trg_wallet_tx_no_update
    BEFORE UPDATE ON wallet_transactions
    FOR EACH ROW EXECUTE FUNCTION prevent_audit_log_modification();
CREATE TRIGGER trg_wallet_tx_no_delete
    BEFORE DELETE ON wallet_transactions
    FOR EACH ROW EXECUTE FUNCTION prevent_audit_log_modification();

-- ── payout_requests ───────────────────────────────────────────
CREATE TABLE payout_requests (
    id           UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_id    UUID          NOT NULL REFERENCES wallets(id) ON DELETE RESTRICT,
    owner_id     UUID          NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    amount       DECIMAL(12,2) NOT NULL CHECK (amount > 0),
    method       VARCHAR(20)   NOT NULL,
    account_info JSONB         NOT NULL DEFAULT '{}',
    status       VARCHAR(30)   NOT NULL DEFAULT 'PENDING',
    admin_note   TEXT,
    processed_by UUID          REFERENCES users(id) ON DELETE SET NULL,
    processed_at TIMESTAMPTZ,
    bank_reference VARCHAR(255),
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT now(),

    CONSTRAINT chk_payout_method CHECK (method IN ('BKASH','NAGAD','BANK')),
    CONSTRAINT chk_payout_status CHECK (status IN (
        'PENDING','APPROVED','PROCESSING','COMPLETED','REJECTED'
    ))
);

CREATE INDEX idx_payout_owner  ON payout_requests(owner_id, created_at DESC);
CREATE INDEX idx_payout_status ON payout_requests(status, created_at DESC);

-- ── escrow_entries ────────────────────────────────────────────
CREATE TABLE escrow_entries (
    id              UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    order_item_id   UUID          NOT NULL UNIQUE REFERENCES order_items(id) ON DELETE RESTRICT,
    seller_id       UUID          NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    amount          DECIMAL(12,2) NOT NULL CHECK (amount >= 0),
    status          VARCHAR(30)   NOT NULL DEFAULT 'HELD',
    hold_until      TIMESTAMPTZ   NOT NULL,
    released_at     TIMESTAMPTZ,
    release_trigger VARCHAR(50),
    clawback_reason TEXT,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ   NOT NULL DEFAULT now(),

    CONSTRAINT chk_escrow_status CHECK (status IN (
        'HELD','RELEASED','CLAWBACKED','DISPUTED'
    )),
    CONSTRAINT chk_release_trigger CHECK (release_trigger IS NULL OR release_trigger IN (
        'AUTO_EXPIRY','ADMIN_MANUAL','NO_RETURN'
    ))
);

CREATE INDEX idx_escrow_seller    ON escrow_entries(seller_id, status);
CREATE INDEX idx_escrow_release   ON escrow_entries(hold_until, status) WHERE status = 'HELD';

CREATE TRIGGER trg_escrow_updated_at
    BEFORE UPDATE ON escrow_entries
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ── refunds ───────────────────────────────────────────────────
CREATE TABLE refunds (
    id              UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id        UUID          NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
    order_item_ids  UUID[]        NOT NULL DEFAULT '{}',
    refund_type     VARCHAR(30)   NOT NULL,
    gateway         VARCHAR(30),
    gateway_refund_id VARCHAR(255),
    amount          DECIMAL(12,2) NOT NULL CHECK (amount > 0),
    reason          TEXT          NOT NULL,
    status          VARCHAR(30)   NOT NULL DEFAULT 'PENDING',
    initiated_by    UUID          NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    processed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT now(),

    CONSTRAINT chk_refund_type CHECK (refund_type IN ('GATEWAY_REVERSAL','WALLET_CREDIT')),
    CONSTRAINT chk_refund_status CHECK (status IN ('PENDING','PROCESSING','COMPLETED','FAILED'))
);

CREATE INDEX idx_refunds_order  ON refunds(order_id);
CREATE INDEX idx_refunds_status ON refunds(status);

-- ── cod_remittances ───────────────────────────────────────────
CREATE TABLE cod_remittances (
    id                UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    courier_id        UUID          NOT NULL,
    remittance_date   DATE          NOT NULL,
    total_amount      DECIMAL(14,2) NOT NULL,
    statement_file_url TEXT,
    status            VARCHAR(30)   NOT NULL DEFAULT 'PENDING',
    reconciled_at     TIMESTAMPTZ,
    reconciled_by     UUID          REFERENCES users(id) ON DELETE SET NULL,
    notes             TEXT,
    created_at        TIMESTAMPTZ   NOT NULL DEFAULT now(),

    CONSTRAINT chk_remittance_status CHECK (status IN ('PENDING','RECONCILED','DISPUTED'))
);

CREATE INDEX idx_cod_remittances_courier ON cod_remittances(courier_id, remittance_date DESC);

-- ── cod_remittance_items ──────────────────────────────────────
CREATE TABLE cod_remittance_items (
    id               UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    remittance_id    UUID          NOT NULL REFERENCES cod_remittances(id) ON DELETE CASCADE,
    consignment_id   UUID,
    order_id         UUID          REFERENCES orders(id) ON DELETE SET NULL,
    expected_amount  DECIMAL(12,2),
    remitted_amount  DECIMAL(12,2) NOT NULL,
    match_status     VARCHAR(20)   NOT NULL DEFAULT 'UNMATCHED',
    discrepancy_amount DECIMAL(12,2),
    resolved_at      TIMESTAMPTZ,
    resolution_note  TEXT,

    CONSTRAINT chk_match_status CHECK (match_status IN (
        'MATCHED','OVER','UNDER','UNMATCHED'
    ))
);

CREATE INDEX idx_cod_items_remittance ON cod_remittance_items(remittance_id);
CREATE INDEX idx_cod_items_order      ON cod_remittance_items(order_id) WHERE order_id IS NOT NULL;
