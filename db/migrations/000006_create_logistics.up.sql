-- ════════════════════════════════════════════════════════════
-- Migration 0006: Logistics Domain
-- ════════════════════════════════════════════════════════════

-- ── couriers ─────────────────────────────────────────────────
CREATE TABLE couriers (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(50)  UNIQUE NOT NULL,
    code        VARCHAR(20)  UNIQUE NOT NULL,
    api_config  JSONB        NOT NULL DEFAULT '{}',
    is_active   BOOLEAN      NOT NULL DEFAULT TRUE,
    supports_cod BOOLEAN     NOT NULL DEFAULT TRUE,
    base_charge DECIMAL(10,2) NOT NULL DEFAULT 0,
    cod_charge_rate DECIMAL(5,2) NOT NULL DEFAULT 1.0,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT chk_courier_name CHECK (name IN ('STEADFAST','PATHAO','REDX','MANUAL'))
);

CREATE TRIGGER trg_couriers_updated_at
    BEFORE UPDATE ON couriers
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Insert default couriers
INSERT INTO couriers (name, code, is_active, supports_cod) VALUES
    ('STEADFAST', 'STF', TRUE, TRUE),
    ('PATHAO',    'PTH', TRUE, TRUE),
    ('REDX',      'RDX', TRUE, TRUE),
    ('MANUAL',    'MNL', TRUE, TRUE);

-- ── consignments ─────────────────────────────────────────────
CREATE TABLE consignments (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    sub_order_id    UUID         NOT NULL UNIQUE REFERENCES sub_orders(id) ON DELETE RESTRICT,
    courier_id      UUID         NOT NULL REFERENCES couriers(id) ON DELETE RESTRICT,
    courier_consignment_id VARCHAR(255),
    tracking_code   VARCHAR(255),
    tracking_url    TEXT,
    status          VARCHAR(50)  NOT NULL DEFAULT 'BOOKED',
    label_url       TEXT,
    cod_amount      DECIMAL(12,2) NOT NULL DEFAULT 0,
    weight_kg       DECIMAL(6,2),
    booked_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    picked_up_at    TIMESTAMPTZ,
    delivered_at    TIMESTAMPTZ,
    returned_at     TIMESTAMPTZ,
    last_synced_at  TIMESTAMPTZ,
    raw_response    JSONB        NOT NULL DEFAULT '{}',
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT chk_consignment_status CHECK (status IN (
        'BOOKED','PICKED_UP','IN_TRANSIT','OUT_FOR_DELIVERY',
        'DELIVERED','PARTIAL_DELIVERED','DELIVERY_FAILED',
        'RETURN_IN_TRANSIT','RETURNED_TO_HUB','CANCELLED','ON_HOLD'
    ))
);

CREATE INDEX idx_consignments_sub_order    ON consignments(sub_order_id);
CREATE INDEX idx_consignments_tracking     ON consignments(tracking_code) WHERE tracking_code IS NOT NULL;
CREATE INDEX idx_consignments_status       ON consignments(status);
CREATE INDEX idx_consignments_courier      ON consignments(courier_id, status);

CREATE TRIGGER trg_consignments_updated_at
    BEFORE UPDATE ON consignments
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ── courier_events (idempotency log) ─────────────────────────
CREATE TABLE courier_events (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    consignment_id  UUID        NOT NULL REFERENCES consignments(id) ON DELETE RESTRICT,
    courier         VARCHAR(30) NOT NULL,
    event_id        VARCHAR(255) UNIQUE NOT NULL,
    courier_status  VARCHAR(100) NOT NULL,
    internal_status VARCHAR(50)  NOT NULL,
    location        TEXT,
    raw_payload     JSONB       NOT NULL DEFAULT '{}',
    processed_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_courier_events_consignment ON courier_events(consignment_id);
CREATE INDEX idx_courier_events_event       ON courier_events(event_id);

-- ── courier_area_mapping ──────────────────────────────────────
CREATE TABLE courier_area_mapping (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    courier_id      UUID         NOT NULL REFERENCES couriers(id) ON DELETE CASCADE,
    district        VARCHAR(100) NOT NULL,
    city            VARCHAR(100),
    courier_city_id INTEGER,
    courier_zone_id INTEGER,
    courier_area_id INTEGER,
    is_available    BOOLEAN      NOT NULL DEFAULT TRUE,
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),

    UNIQUE(courier_id, district, city)
);

CREATE INDEX idx_area_mapping_courier   ON courier_area_mapping(courier_id);
CREATE INDEX idx_area_mapping_district  ON courier_area_mapping(district);
