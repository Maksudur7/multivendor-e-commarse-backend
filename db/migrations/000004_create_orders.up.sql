-- ════════════════════════════════════════════════════════════
-- Migration 0004: Order & Cart Domain
-- ════════════════════════════════════════════════════════════

-- ── coupons ───────────────────────────────────────────────────
CREATE TABLE coupons (
    id                UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    code              VARCHAR(50)   UNIQUE NOT NULL,
    name              VARCHAR(255)  NOT NULL,
    description       TEXT,
    discount_type     VARCHAR(30)   NOT NULL,
    discount_value    DECIMAL(12,2) NOT NULL CHECK (discount_value > 0),
    max_discount_cap  DECIMAL(12,2),
    min_order_amount  DECIMAL(12,2) NOT NULL DEFAULT 0,
    applicable_to     VARCHAR(30)   NOT NULL DEFAULT 'ALL',
    applicable_ids    UUID[]        NOT NULL DEFAULT '{}',
    excluded_ids      UUID[]        NOT NULL DEFAULT '{}',
    user_type_allowed VARCHAR(20)   NOT NULL DEFAULT 'ALL',
    max_total_uses    INTEGER,
    max_per_user      INTEGER       NOT NULL DEFAULT 1,
    total_used_count  INTEGER       NOT NULL DEFAULT 0,
    is_active         BOOLEAN       NOT NULL DEFAULT TRUE,
    starts_at         TIMESTAMPTZ,
    expires_at        TIMESTAMPTZ,
    created_by        UUID          NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at        TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ   NOT NULL DEFAULT now(),

    CONSTRAINT chk_coupon_type CHECK (discount_type IN ('FLAT','PERCENTAGE','FREE_SHIPPING')),
    CONSTRAINT chk_coupon_applicable CHECK (applicable_to IN ('ALL','CATEGORY','PRODUCT','SELLER'))
);

CREATE INDEX idx_coupons_code   ON coupons(code);
CREATE INDEX idx_coupons_active ON coupons(is_active, expires_at);

CREATE TRIGGER trg_coupons_updated_at
    BEFORE UPDATE ON coupons
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ── coupon_usages ─────────────────────────────────────────────
CREATE TABLE coupon_usages (
    id               UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    coupon_id        UUID          NOT NULL REFERENCES coupons(id) ON DELETE RESTRICT,
    user_id          UUID          NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    order_id         UUID,
    discount_applied DECIMAL(12,2) NOT NULL,
    used_at          TIMESTAMPTZ   NOT NULL DEFAULT now(),

    UNIQUE(coupon_id, user_id, order_id)
);

CREATE INDEX idx_coupon_usages_user   ON coupon_usages(user_id);
CREATE INDEX idx_coupon_usages_coupon ON coupon_usages(coupon_id);

-- ── carts ─────────────────────────────────────────────────────
CREATE TABLE carts (
    id         UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID         UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    session_id VARCHAR(255) UNIQUE,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT chk_cart_owner CHECK (
        user_id IS NOT NULL OR session_id IS NOT NULL
    )
);

CREATE INDEX idx_carts_user    ON carts(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX idx_carts_session ON carts(session_id) WHERE session_id IS NOT NULL;

CREATE TRIGGER trg_carts_updated_at
    BEFORE UPDATE ON carts
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ── cart_items ────────────────────────────────────────────────
CREATE TABLE cart_items (
    id           UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    cart_id      UUID          NOT NULL REFERENCES carts(id) ON DELETE CASCADE,
    variant_id   UUID          NOT NULL REFERENCES product_variants(id) ON DELETE CASCADE,
    quantity     INTEGER       NOT NULL CHECK (quantity > 0),
    price_at_add DECIMAL(12,2) NOT NULL,
    added_at     TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ   NOT NULL DEFAULT now(),

    UNIQUE(cart_id, variant_id)
);

CREATE INDEX idx_cart_items_cart ON cart_items(cart_id);

-- ── orders ────────────────────────────────────────────────────
CREATE TABLE orders (
    id               UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    order_number     VARCHAR(50)   UNIQUE NOT NULL,
    customer_id      UUID          NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    reseller_id      UUID          REFERENCES users(id) ON DELETE SET NULL,
    order_type       VARCHAR(20)   NOT NULL DEFAULT 'CUSTOMER',
    status           VARCHAR(50)   NOT NULL DEFAULT 'PENDING_PAYMENT',
    address_id       UUID          REFERENCES addresses(id) ON DELETE SET NULL,
    address_snapshot JSONB         NOT NULL DEFAULT '{}',
    payment_method   VARCHAR(30)   NOT NULL,
    coupon_id        UUID          REFERENCES coupons(id) ON DELETE SET NULL,
    coupon_code      VARCHAR(50),
    coupon_discount  DECIMAL(12,2) NOT NULL DEFAULT 0,
    subtotal         DECIMAL(12,2) NOT NULL,
    total_shipping   DECIMAL(12,2) NOT NULL DEFAULT 0,
    discount_total   DECIMAL(12,2) NOT NULL DEFAULT 0,
    grand_total      DECIMAL(12,2) NOT NULL,
    currency         VARCHAR(5)    NOT NULL DEFAULT 'BDT',
    customer_note    TEXT,
    device_fingerprint VARCHAR(255),
    is_white_label   BOOLEAN       NOT NULL DEFAULT FALSE,
    cancel_reason    TEXT,
    cancelled_by     UUID          REFERENCES users(id) ON DELETE SET NULL,
    cancelled_at     TIMESTAMPTZ,
    created_at       TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ   NOT NULL DEFAULT now(),

    CONSTRAINT chk_order_type CHECK (order_type IN ('CUSTOMER','RESELLER')),
    CONSTRAINT chk_order_status CHECK (status IN (
        'PENDING_PAYMENT','PAYMENT_CONFIRMED','PROCESSING','READY_TO_SHIP',
        'DISPATCHED','IN_TRANSIT','DELIVERED','DELIVERY_FAILED',
        'RETURN_REQUESTED','RETURN_APPROVED','RETURN_IN_TRANSIT',
        'RETURNED','RETURNED_TO_HUB','CANCELLED'
    )),
    CONSTRAINT chk_order_payment_method CHECK (payment_method IN (
        'SSLCOMMERZ','BKASH','NAGAD','COD','WALLET'
    ))
);

CREATE INDEX idx_orders_customer    ON orders(customer_id);
CREATE INDEX idx_orders_reseller    ON orders(reseller_id) WHERE reseller_id IS NOT NULL;
CREATE INDEX idx_orders_status      ON orders(status);
CREATE INDEX idx_orders_number      ON orders(order_number);
CREATE INDEX idx_orders_created     ON orders(created_at DESC);
CREATE INDEX idx_orders_payment     ON orders(payment_method);

CREATE TRIGGER trg_orders_updated_at
    BEFORE UPDATE ON orders
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ── sub_orders ────────────────────────────────────────────────
CREATE TABLE sub_orders (
    id               UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id         UUID          NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
    sub_order_number VARCHAR(60)   UNIQUE NOT NULL,
    seller_id        UUID          NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    status           VARCHAR(50)   NOT NULL DEFAULT 'PAYMENT_CONFIRMED',
    courier_id       UUID,
    consignment_id   UUID,
    tracking_number  VARCHAR(255),
    tracking_url     TEXT,
    packing_slip_url TEXT,
    dispatched_at    TIMESTAMPTZ,
    delivered_at     TIMESTAMPTZ,
    return_deadline  TIMESTAMPTZ,
    shipping_fee     DECIMAL(12,2) NOT NULL DEFAULT 0,
    shipping_origin  VARCHAR(20)   NOT NULL DEFAULT 'LOCAL_BD',
    created_at       TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX idx_sub_orders_order  ON sub_orders(order_id);
CREATE INDEX idx_sub_orders_seller ON sub_orders(seller_id, created_at DESC);
CREATE INDEX idx_sub_orders_status ON sub_orders(status);

CREATE TRIGGER trg_sub_orders_updated_at
    BEFORE UPDATE ON sub_orders
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ── order_items ───────────────────────────────────────────────
CREATE TABLE order_items (
    id                  UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id            UUID          NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
    sub_order_id        UUID          NOT NULL REFERENCES sub_orders(id) ON DELETE RESTRICT,
    variant_id          UUID          NOT NULL REFERENCES product_variants(id) ON DELETE RESTRICT,
    product_snapshot    JSONB         NOT NULL DEFAULT '{}',
    seller_id           UUID          NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    quantity            INTEGER       NOT NULL CHECK (quantity > 0),
    unit_price          DECIMAL(12,2) NOT NULL,
    original_price      DECIMAL(12,2) NOT NULL,
    wholesale_price     DECIMAL(12,2),
    reseller_sell_price DECIMAL(12,2),
    reseller_profit     DECIMAL(12,2),
    platform_commission DECIMAL(12,2) NOT NULL DEFAULT 0,
    commission_rate     DECIMAL(5,2)  NOT NULL DEFAULT 0,
    gateway_fee         DECIMAL(12,2) NOT NULL DEFAULT 0,
    shipping_fee        DECIMAL(12,2) NOT NULL DEFAULT 0,
    seller_net_payout   DECIMAL(12,2) NOT NULL DEFAULT 0,
    escrow_released     BOOLEAN       NOT NULL DEFAULT FALSE,
    escrow_released_at  TIMESTAMPTZ,
    line_total          DECIMAL(12,2) NOT NULL,
    created_at          TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX idx_order_items_order     ON order_items(order_id);
CREATE INDEX idx_order_items_sub_order ON order_items(sub_order_id);
CREATE INDEX idx_order_items_seller    ON order_items(seller_id);
CREATE INDEX idx_order_items_variant   ON order_items(variant_id);
CREATE INDEX idx_order_items_escrow    ON order_items(escrow_released, created_at)
    WHERE escrow_released = FALSE;

-- ── order_status_history ─────────────────────────────────────
CREATE TABLE order_status_history (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id     UUID        NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    sub_order_id UUID        REFERENCES sub_orders(id) ON DELETE CASCADE,
    status       VARCHAR(50) NOT NULL,
    description  TEXT,
    location     TEXT,
    changed_by   UUID        REFERENCES users(id) ON DELETE SET NULL,
    source       VARCHAR(20) NOT NULL DEFAULT 'SYSTEM',
    timestamp    TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT chk_history_source CHECK (source IN ('SYSTEM','ADMIN','SELLER','COURIER_WEBHOOK'))
);

CREATE INDEX idx_status_history_order ON order_status_history(order_id, timestamp DESC);
CREATE INDEX idx_status_history_sub   ON order_status_history(sub_order_id) WHERE sub_order_id IS NOT NULL;

-- ── reseller_orders ───────────────────────────────────────────
CREATE TABLE reseller_orders (
    id                   UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id             UUID          NOT NULL UNIQUE REFERENCES orders(id) ON DELETE RESTRICT,
    reseller_id          UUID          NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    end_customer_name    VARCHAR(255)  NOT NULL,
    end_customer_phone   VARCHAR(15)   NOT NULL,
    end_customer_address JSONB         NOT NULL DEFAULT '{}',
    total_wholesale_cost DECIMAL(12,2) NOT NULL,
    total_sell_price     DECIMAL(12,2) NOT NULL,
    total_profit         DECIMAL(12,2) NOT NULL,
    profit_status        VARCHAR(30)   NOT NULL DEFAULT 'PENDING',
    is_white_label       BOOLEAN       NOT NULL DEFAULT TRUE,
    packing_note         TEXT,
    reseller_brand_name  VARCHAR(255),
    created_at           TIMESTAMPTZ   NOT NULL DEFAULT now(),

    CONSTRAINT chk_profit_status CHECK (profit_status IN (
        'PENDING','AVAILABLE','WITHDRAWN','CANCELLED'
    ))
);

CREATE INDEX idx_reseller_orders_reseller ON reseller_orders(reseller_id);
CREATE INDEX idx_reseller_orders_status   ON reseller_orders(profit_status);
