-- ════════════════════════════════════════════════════════════
-- Migration 0002: Catalog & Product Domain
-- ════════════════════════════════════════════════════════════

-- ── categories ───────────────────────────────────────────────
CREATE TABLE categories (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    parent_id        UUID         REFERENCES categories(id) ON DELETE RESTRICT,
    name             VARCHAR(255) NOT NULL,
    name_bn          VARCHAR(255),
    slug             VARCHAR(255) UNIQUE NOT NULL,
    description      TEXT,
    image_url        TEXT,
    icon_url         TEXT,
    banner_url       TEXT,
    sort_order       INTEGER      NOT NULL DEFAULT 0,
    is_active        BOOLEAN      NOT NULL DEFAULT TRUE,
    is_leaf          BOOLEAN      NOT NULL DEFAULT FALSE,
    depth            INTEGER      NOT NULL DEFAULT 0,
    path             TEXT         NOT NULL DEFAULT '',
    meta_title       VARCHAR(255),
    meta_description TEXT,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_categories_parent   ON categories(parent_id);
CREATE INDEX idx_categories_slug     ON categories(slug);
CREATE INDEX idx_categories_path     ON categories(path);
CREATE INDEX idx_categories_active   ON categories(is_active, sort_order);

CREATE TRIGGER trg_categories_updated_at
    BEFORE UPDATE ON categories
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ── category_commission_rates ─────────────────────────────────
CREATE TABLE category_commission_rates (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id     UUID         NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    commission_rate DECIMAL(5,2) NOT NULL CHECK (commission_rate >= 0 AND commission_rate <= 100),
    created_by      UUID         REFERENCES users(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),

    UNIQUE(category_id)
);

-- ── brands ───────────────────────────────────────────────────
CREATE TABLE brands (
    id                 UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name               VARCHAR(255) NOT NULL,
    slug               VARCHAR(255) UNIQUE NOT NULL,
    logo_url           TEXT,
    country_of_origin  VARCHAR(100),
    website_url        TEXT,
    is_verified        BOOLEAN      NOT NULL DEFAULT FALSE,
    is_active          BOOLEAN      NOT NULL DEFAULT TRUE,
    created_by         UUID         REFERENCES users(id) ON DELETE SET NULL,
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_brands_slug     ON brands(slug);
CREATE INDEX idx_brands_verified ON brands(is_verified);

CREATE TRIGGER trg_brands_updated_at
    BEFORE UPDATE ON brands
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ── attribute_sets ───────────────────────────────────────────
CREATE TABLE attribute_sets (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255) NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- ── attributes ───────────────────────────────────────────────
CREATE TABLE attributes (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    set_id         UUID        NOT NULL REFERENCES attribute_sets(id) ON DELETE CASCADE,
    name           VARCHAR(255) NOT NULL,
    name_bn        VARCHAR(255),
    code           VARCHAR(100) UNIQUE NOT NULL,
    type           VARCHAR(30)  NOT NULL DEFAULT 'TEXT',
    is_required    BOOLEAN      NOT NULL DEFAULT FALSE,
    is_variant     BOOLEAN      NOT NULL DEFAULT FALSE,
    is_filterable  BOOLEAN      NOT NULL DEFAULT FALSE,
    is_searchable  BOOLEAN      NOT NULL DEFAULT FALSE,
    sort_order     INTEGER      NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT chk_attribute_type CHECK (type IN (
        'TEXT','SELECT','MULTI_SELECT','COLOR','NUMBER','BOOLEAN'
    ))
);

CREATE INDEX idx_attributes_set  ON attributes(set_id);
CREATE INDEX idx_attributes_code ON attributes(code);

-- ── attribute_options ─────────────────────────────────────────
CREATE TABLE attribute_options (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    attribute_id  UUID         NOT NULL REFERENCES attributes(id) ON DELETE CASCADE,
    value         VARCHAR(255) NOT NULL,
    value_bn      VARCHAR(255),
    display_label VARCHAR(255),
    hex_color     VARCHAR(7),
    sort_order    INTEGER      NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_attr_options_attr ON attribute_options(attribute_id);

-- ── seller_profiles ──────────────────────────────────────────
CREATE TABLE seller_profiles (
    id                  UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID         NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    shop_name           VARCHAR(255) NOT NULL,
    shop_slug           VARCHAR(255) UNIQUE NOT NULL,
    shop_description    TEXT,
    shop_logo_url       TEXT,
    shop_banner_url     TEXT,
    shop_category       VARCHAR(100),
    status              VARCHAR(30)  NOT NULL DEFAULT 'PENDING',
    trade_license_no    VARCHAR(100),
    trade_license_url   TEXT,
    nid_number          VARCHAR(255),
    bank_info           JSONB,
    bkash_number        VARCHAR(15),
    seller_rating       DECIMAL(3,2) NOT NULL DEFAULT 0,
    total_sales         INTEGER      NOT NULL DEFAULT 0,
    total_orders        INTEGER      NOT NULL DEFAULT 0,
    return_rate         DECIMAL(5,2) NOT NULL DEFAULT 0,
    commission_override DECIMAL(5,2),
    joined_at           TIMESTAMPTZ  NOT NULL DEFAULT now(),
    suspended_reason    TEXT,
    suspended_by        UUID         REFERENCES users(id) ON DELETE SET NULL,
    suspended_at        TIMESTAMPTZ,
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT chk_seller_status CHECK (status IN (
        'PENDING','ACTIVE','SUSPENDED','TERMINATED'
    ))
);

CREATE INDEX idx_seller_user   ON seller_profiles(user_id);
CREATE INDEX idx_seller_status ON seller_profiles(status);
CREATE INDEX idx_seller_slug   ON seller_profiles(shop_slug);

CREATE TRIGGER trg_seller_updated_at
    BEFORE UPDATE ON seller_profiles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ── products ─────────────────────────────────────────────────
CREATE TABLE products (
    id                   UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    seller_id            UUID         NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    category_id          UUID         NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,
    brand_id             UUID         REFERENCES brands(id) ON DELETE SET NULL,
    product_type         VARCHAR(20)  NOT NULL DEFAULT 'SIMPLE',
    name                 TEXT         NOT NULL,
    name_bn              TEXT,
    slug                 VARCHAR(500) UNIQUE NOT NULL,
    short_description    TEXT,
    description          TEXT,
    description_bn       TEXT,
    shipping_origin      VARCHAR(20)  NOT NULL DEFAULT 'LOCAL_BD',
    local_delivery_days  VARCHAR(20)  NOT NULL DEFAULT '2-3',
    china_delivery_days  VARCHAR(20)  NOT NULL DEFAULT '10-18',
    return_policy_days   INTEGER      NOT NULL DEFAULT 7,
    warranty_info        TEXT,
    is_cod_eligible      BOOLEAN      NOT NULL DEFAULT TRUE,
    status               VARCHAR(30)  NOT NULL DEFAULT 'DRAFT',
    rejection_reason     TEXT,
    rejection_code       VARCHAR(50),
    reviewed_by          UUID         REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at          TIMESTAMPTZ,
    is_featured          BOOLEAN      NOT NULL DEFAULT FALSE,
    is_bestseller        BOOLEAN      NOT NULL DEFAULT FALSE,
    is_new_arrival       BOOLEAN      NOT NULL DEFAULT FALSE,
    sort_score           DECIMAL(10,4) NOT NULL DEFAULT 0,
    average_rating       DECIMAL(3,2) NOT NULL DEFAULT 0,
    review_count         INTEGER      NOT NULL DEFAULT 0,
    view_count           BIGINT       NOT NULL DEFAULT 0,
    total_sales          INTEGER      NOT NULL DEFAULT 0,
    meta_title           VARCHAR(255),
    meta_description     TEXT,
    tags                 TEXT[]       NOT NULL DEFAULT '{}',
    deleted_at           TIMESTAMPTZ,
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT chk_product_type CHECK (product_type IN ('SIMPLE','CONFIGURABLE','BUNDLE')),
    CONSTRAINT chk_product_status CHECK (status IN (
        'DRAFT','PENDING_REVIEW','APPROVED','REJECTED','INACTIVE','DEACTIVATED'
    )),
    CONSTRAINT chk_shipping_origin CHECK (shipping_origin IN ('LOCAL_BD','CHINA_DIRECT'))
);

CREATE INDEX idx_products_seller       ON products(seller_id);
CREATE INDEX idx_products_category     ON products(category_id);
CREATE INDEX idx_products_status       ON products(status);
CREATE INDEX idx_products_slug         ON products(slug);
CREATE INDEX idx_products_origin       ON products(shipping_origin);
CREATE INDEX idx_products_featured     ON products(is_featured, status) WHERE is_featured = TRUE;
CREATE INDEX idx_products_bestseller   ON products(is_bestseller, status) WHERE is_bestseller = TRUE;
CREATE INDEX idx_products_new_arrival  ON products(is_new_arrival, status) WHERE is_new_arrival = TRUE;
CREATE INDEX idx_products_not_deleted  ON products(created_at) WHERE deleted_at IS NULL;
CREATE INDEX idx_products_sort         ON products(sort_score DESC);
CREATE INDEX idx_products_tags         ON products USING GIN(tags);

CREATE TRIGGER trg_products_updated_at
    BEFORE UPDATE ON products
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ── product_variants ──────────────────────────────────────────
CREATE TABLE product_variants (
    id                  UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id          UUID          NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    sku                 VARCHAR(255)  UNIQUE NOT NULL,
    barcode             VARCHAR(255),
    attributes          JSONB         NOT NULL DEFAULT '{}',
    price               DECIMAL(12,2) NOT NULL CHECK (price >= 0),
    compare_at_price    DECIMAL(12,2),
    wholesale_price     DECIMAL(12,2),
    cost_price          DECIMAL(12,2),
    weight_grams        INTEGER,
    length_cm           DECIMAL(8,2),
    width_cm            DECIMAL(8,2),
    height_cm           DECIMAL(8,2),
    is_active           BOOLEAN       NOT NULL DEFAULT TRUE,
    low_stock_threshold INTEGER       NOT NULL DEFAULT 10,
    position            INTEGER       NOT NULL DEFAULT 0,
    commission_override DECIMAL(5,2),
    created_at          TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX idx_variants_product ON product_variants(product_id);
CREATE INDEX idx_variants_sku     ON product_variants(sku);
CREATE INDEX idx_variants_active  ON product_variants(product_id, is_active);

CREATE TRIGGER trg_variants_updated_at
    BEFORE UPDATE ON product_variants
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ── price_tiers ───────────────────────────────────────────────
CREATE TABLE price_tiers (
    id           UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id   UUID          NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    variant_id   UUID          REFERENCES product_variants(id) ON DELETE CASCADE,
    min_quantity INTEGER       NOT NULL CHECK (min_quantity >= 1),
    max_quantity INTEGER,
    price        DECIMAL(12,2) NOT NULL CHECK (price >= 0),
    tier_type    VARCHAR(20)   NOT NULL DEFAULT 'WHOLESALE',
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT now(),

    CONSTRAINT chk_tier_type CHECK (tier_type IN ('WHOLESALE','RESELLER','VIP')),
    CONSTRAINT chk_price_tier_qty CHECK (max_quantity IS NULL OR max_quantity > min_quantity)
);

CREATE INDEX idx_price_tiers_product ON price_tiers(product_id, tier_type);
CREATE INDEX idx_price_tiers_variant ON price_tiers(variant_id) WHERE variant_id IS NOT NULL;

-- ── product_media ─────────────────────────────────────────────
CREATE TABLE product_media (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id        UUID        NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    variant_id        UUID        REFERENCES product_variants(id) ON DELETE CASCADE,
    media_type        VARCHAR(10) NOT NULL DEFAULT 'IMAGE',
    url               TEXT        NOT NULL,
    thumbnail_url     TEXT,
    medium_url        TEXT,
    large_url         TEXT,
    alt_text          VARCHAR(500),
    sort_order        INTEGER     NOT NULL DEFAULT 0,
    is_primary        BOOLEAN     NOT NULL DEFAULT FALSE,
    is_reseller_asset BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT chk_media_type CHECK (media_type IN ('IMAGE','VIDEO'))
);

CREATE INDEX idx_media_product   ON product_media(product_id, sort_order);
CREATE INDEX idx_media_variant   ON product_media(variant_id) WHERE variant_id IS NOT NULL;
CREATE INDEX idx_media_primary   ON product_media(product_id) WHERE is_primary = TRUE;
CREATE INDEX idx_media_reseller  ON product_media(product_id) WHERE is_reseller_asset = TRUE;
