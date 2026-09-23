-- ════════════════════════════════════════════════════════════
-- Migration 0003: Inventory Domain
-- ════════════════════════════════════════════════════════════

-- ── warehouses ───────────────────────────────────────────────
CREATE TABLE warehouses (
    id         UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(255) NOT NULL,
    code       VARCHAR(50)  UNIQUE NOT NULL,
    type       VARCHAR(20)  NOT NULL DEFAULT 'LOCAL_BD',
    address    TEXT,
    city       VARCHAR(100),
    country    VARCHAR(100) NOT NULL DEFAULT 'Bangladesh',
    is_active  BOOLEAN      NOT NULL DEFAULT TRUE,
    is_default BOOLEAN      NOT NULL DEFAULT FALSE,
    manager_id UUID         REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT chk_warehouse_type CHECK (type IN ('LOCAL_BD','CHINA'))
);

CREATE INDEX idx_warehouse_type ON warehouses(type, is_active);

-- ── inventory ─────────────────────────────────────────────────
CREATE TABLE inventory (
    id                  UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    variant_id          UUID    NOT NULL REFERENCES product_variants(id) ON DELETE CASCADE,
    warehouse_id        UUID    NOT NULL REFERENCES warehouses(id) ON DELETE RESTRICT,
    quantity_on_hand    INTEGER NOT NULL DEFAULT 0 CHECK (quantity_on_hand >= 0),
    quantity_reserved   INTEGER NOT NULL DEFAULT 0 CHECK (quantity_reserved >= 0),
    quantity_in_transit INTEGER NOT NULL DEFAULT 0 CHECK (quantity_in_transit >= 0),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE(variant_id, warehouse_id),
    CONSTRAINT chk_reserved_lte_onhand CHECK (quantity_reserved <= quantity_on_hand)
);

CREATE INDEX idx_inventory_variant   ON inventory(variant_id);
CREATE INDEX idx_inventory_warehouse ON inventory(warehouse_id);
CREATE INDEX idx_inventory_low_stock ON inventory(variant_id)
    WHERE quantity_on_hand <= 10;

CREATE TRIGGER trg_inventory_updated_at
    BEFORE UPDATE ON inventory
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ── inventory_reservations ────────────────────────────────────
CREATE TABLE inventory_reservations (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    variant_id   UUID        NOT NULL REFERENCES product_variants(id) ON DELETE CASCADE,
    warehouse_id UUID        NOT NULL REFERENCES warehouses(id) ON DELETE RESTRICT,
    order_id     UUID,
    quantity     INTEGER     NOT NULL CHECK (quantity > 0),
    expires_at   TIMESTAMPTZ NOT NULL,
    is_confirmed BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_reservations_variant  ON inventory_reservations(variant_id);
CREATE INDEX idx_reservations_order    ON inventory_reservations(order_id) WHERE order_id IS NOT NULL;
CREATE INDEX idx_reservations_expires  ON inventory_reservations(expires_at) WHERE is_confirmed = FALSE;

-- ── inventory_movements (immutable ledger) ────────────────────
CREATE TABLE inventory_movements (
    id             UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    variant_id     UUID         NOT NULL REFERENCES product_variants(id) ON DELETE RESTRICT,
    warehouse_id   UUID         NOT NULL REFERENCES warehouses(id) ON DELETE RESTRICT,
    movement_type  VARCHAR(50)  NOT NULL,
    quantity       INTEGER      NOT NULL,
    quantity_before INTEGER     NOT NULL,
    quantity_after  INTEGER     NOT NULL,
    reference_type VARCHAR(50),
    reference_id   UUID,
    note           TEXT,
    performed_by   UUID         REFERENCES users(id) ON DELETE SET NULL,
    timestamp      TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT chk_movement_type CHECK (movement_type IN (
        'PURCHASE_RECEIVED','RETURN_RESTOCKED','MANUAL_ADJUSTMENT_IN','CHINA_BATCH_ARRIVED',
        'SALE_COMMITTED','SALE_COMPLETED','DAMAGE_WRITE_OFF','MANUAL_ADJUSTMENT_OUT',
        'RESERVATION_CREATED','RESERVATION_RELEASED','TRANSFER_OUT','TRANSFER_IN'
    ))
) PARTITION BY RANGE (timestamp);

CREATE TABLE inventory_movements_2024 PARTITION OF inventory_movements
    FOR VALUES FROM ('2024-01-01') TO ('2025-01-01');
CREATE TABLE inventory_movements_2025 PARTITION OF inventory_movements
    FOR VALUES FROM ('2025-01-01') TO ('2026-01-01');
CREATE TABLE inventory_movements_2026 PARTITION OF inventory_movements
    FOR VALUES FROM ('2026-01-01') TO ('2027-01-01');

CREATE INDEX idx_inv_moves_variant   ON inventory_movements(variant_id, timestamp DESC);
CREATE INDEX idx_inv_moves_warehouse ON inventory_movements(warehouse_id, timestamp DESC);
CREATE INDEX idx_inv_moves_reference ON inventory_movements(reference_type, reference_id)
    WHERE reference_id IS NOT NULL;

-- Prevent modifications to inventory_movements
CREATE TRIGGER trg_inv_movements_no_update
    BEFORE UPDATE ON inventory_movements
    FOR EACH ROW EXECUTE FUNCTION prevent_audit_log_modification();
CREATE TRIGGER trg_inv_movements_no_delete
    BEFORE DELETE ON inventory_movements
    FOR EACH ROW EXECUTE FUNCTION prevent_audit_log_modification();

-- ── china_batches ─────────────────────────────────────────────
CREATE TABLE china_batches (
    id                UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_code        VARCHAR(50)   UNIQUE NOT NULL,
    batch_type        VARCHAR(20)   NOT NULL DEFAULT 'SEA',
    supplier_name     VARCHAR(255),
    supplier_country  VARCHAR(100)  NOT NULL DEFAULT 'China',
    status            VARCHAR(50)   NOT NULL DEFAULT 'SOURCING',
    total_items       INTEGER       NOT NULL DEFAULT 0,
    total_units       INTEGER       NOT NULL DEFAULT 0,
    total_cny_cost    DECIMAL(14,2),
    total_freight_cost DECIMAL(14,2),
    total_customs_duty DECIMAL(14,2),
    total_other_costs  DECIMAL(14,2),
    total_landed_cost  DECIMAL(14,2),
    currency_rate      DECIMAL(10,4),
    po_number         VARCHAR(100),
    tracking_number   VARCHAR(255),
    departure_date    DATE,
    estimated_arrival DATE,
    actual_arrival    DATE,
    customs_entry_no  VARCHAR(100),
    warehouse_id      UUID          REFERENCES warehouses(id) ON DELETE SET NULL,
    notes             TEXT,
    created_by        UUID          NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at        TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ   NOT NULL DEFAULT now(),

    CONSTRAINT chk_batch_type CHECK (batch_type IN ('SEA','AIR')),
    CONSTRAINT chk_batch_status CHECK (status IN (
        'SOURCING','ORDERED','IN_PRODUCTION','READY_TO_SHIP',
        'IN_TRANSIT_SEA','IN_TRANSIT_AIR','ARRIVED_PORT',
        'CUSTOMS_CLEARANCE','CUSTOMS_HELD','IN_TRANSIT_BD',
        'ARRIVED_WAREHOUSE','COMPLETED'
    ))
);

CREATE INDEX idx_china_batches_status ON china_batches(status);

CREATE TRIGGER trg_china_batches_updated_at
    BEFORE UPDATE ON china_batches
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ── china_batch_items ─────────────────────────────────────────
CREATE TABLE china_batch_items (
    id                UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id          UUID          NOT NULL REFERENCES china_batches(id) ON DELETE CASCADE,
    product_id        UUID          NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    variant_id        UUID          NOT NULL REFERENCES product_variants(id) ON DELETE RESTRICT,
    quantity_ordered  INTEGER       NOT NULL CHECK (quantity_ordered > 0),
    quantity_received INTEGER       NOT NULL DEFAULT 0,
    unit_cny_cost     DECIMAL(10,2) NOT NULL,
    unit_landed_cost  DECIMAL(10,2)
);

CREATE INDEX idx_batch_items_batch ON china_batch_items(batch_id);
