-- Storage statistics table for global stats
CREATE TABLE IF NOT EXISTS storage_stats (
    id VARCHAR(50) PRIMARY KEY,
    created_at TIMESTAMPTZ,
    modified_at TIMESTAMPTZ,
    version INTEGER DEFAULT 0,
    tenant_id VARCHAR(50),
    partition_id VARCHAR(50),
    access_id VARCHAR(50),
    deleted_at TIMESTAMPTZ,

    total_bytes BIGINT DEFAULT 0,
    file_count INTEGER DEFAULT 0,
    user_count INTEGER DEFAULT 0,
    public_bytes BIGINT DEFAULT 0,
    private_bytes BIGINT DEFAULT 0,
    metadata JSONB
);

CREATE INDEX IF NOT EXISTS idx_storage_stats_created_at ON storage_stats (created_at);

-- Materialized view for daily snapshots (refreshed daily)
CREATE TABLE IF NOT EXISTS storage_daily_snapshot (
    record_date DATE PRIMARY KEY,
    total_bytes BIGINT,
    file_count INTEGER,
    user_count INTEGER,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
