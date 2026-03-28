-- Links table: stores redirect links with affiliate tracking metadata.
CREATE TABLE IF NOT EXISTS links (
    id           VARCHAR(50) PRIMARY KEY,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    modified_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version      INTEGER NOT NULL DEFAULT 1,
    tenant_id    VARCHAR(50) NOT NULL DEFAULT '',
    partition_id VARCHAR(50) NOT NULL DEFAULT '',
    access_id    VARCHAR(50) NOT NULL DEFAULT '',
    deleted_at   TIMESTAMPTZ,

    slug             VARCHAR(50) NOT NULL,
    destination_url  TEXT NOT NULL,
    title            VARCHAR(500) DEFAULT '',
    affiliate_id     VARCHAR(50) DEFAULT '',

    campaign VARCHAR(250) DEFAULT '',
    source   VARCHAR(250) DEFAULT '',
    medium   VARCHAR(250) DEFAULT '',
    content  VARCHAR(250) DEFAULT '',
    term     VARCHAR(250) DEFAULT '',

    tags       JSONB DEFAULT '{}',
    max_clicks BIGINT DEFAULT 0,
    expires_at TIMESTAMPTZ,

    state              SMALLINT NOT NULL DEFAULT 1,
    click_count        BIGINT NOT NULL DEFAULT 0,
    unique_click_count BIGINT NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_links_slug ON links (slug);
CREATE INDEX IF NOT EXISTS idx_links_affiliate_id ON links (affiliate_id);
CREATE INDEX IF NOT EXISTS idx_links_campaign ON links (campaign);
CREATE INDEX IF NOT EXISTS idx_links_state ON links (state);


-- Clicks table: append-only telemetry for each redirect event.
CREATE TABLE IF NOT EXISTS clicks (
    id           VARCHAR(50) PRIMARY KEY,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    modified_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version      INTEGER NOT NULL DEFAULT 1,
    tenant_id    VARCHAR(50) NOT NULL DEFAULT '',
    partition_id VARCHAR(50) NOT NULL DEFAULT '',
    access_id    VARCHAR(50) NOT NULL DEFAULT '',
    deleted_at   TIMESTAMPTZ,

    link_id         VARCHAR(50) NOT NULL,
    affiliate_id    VARCHAR(50) DEFAULT '',
    slug            VARCHAR(50) DEFAULT '',

    ip_address      VARCHAR(45) DEFAULT '',
    user_agent      TEXT DEFAULT '',
    referer         TEXT DEFAULT '',
    accept_language VARCHAR(250) DEFAULT '',

    country     VARCHAR(10) DEFAULT '',
    city        VARCHAR(100) DEFAULT '',
    device_type SMALLINT DEFAULT 0,
    browser     VARCHAR(100) DEFAULT '',
    os          VARCHAR(100) DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_click_link_created ON clicks (link_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_click_affiliate ON clicks (affiliate_id);
CREATE INDEX IF NOT EXISTS idx_click_slug ON clicks (slug);
CREATE INDEX IF NOT EXISTS idx_click_ip_link ON clicks (link_id, ip_address, created_at DESC);

-- Partial index for recent clicks — supports fast uniqueness lookups within a 24h window.
-- Only indexes rows from the last 7 days for compact index size on high-volume tables.
-- This index should be recreated periodically via scheduled maintenance if using static WHERE clause,
-- or rely on the composite index above for the general case.
