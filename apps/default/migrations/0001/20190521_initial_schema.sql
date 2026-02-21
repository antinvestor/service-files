CREATE TABLE IF NOT EXISTS media_metadata (
    id VARCHAR(50) PRIMARY KEY,
    created_at TIMESTAMPTZ,
    modified_at TIMESTAMPTZ,
    version INTEGER DEFAULT 0,
    tenant_id VARCHAR(50),
    partition_id VARCHAR(50),
    access_id VARCHAR(50),
    deleted_at TIMESTAMPTZ,

    owner_id TEXT,
    parent_id TEXT,
    name TEXT,
    ext TEXT,
    size BIGINT,
    origin_ts BIGINT,
    public BOOLEAN DEFAULT FALSE,
    mimetype TEXT,
    server_name TEXT,
    hash TEXT,
    bucket_name TEXT,
    provider TEXT,
    properties JSONB
);

CREATE INDEX IF NOT EXISTS idx_media_metadata_owner_id ON media_metadata (owner_id);
CREATE INDEX IF NOT EXISTS idx_media_metadata_hash ON media_metadata (hash);
CREATE INDEX IF NOT EXISTS idx_media_metadata_parent_id ON media_metadata (parent_id);
CREATE INDEX IF NOT EXISTS idx_media_metadata_created_at ON media_metadata (created_at);

CREATE TABLE IF NOT EXISTS media_audit (
    id VARCHAR(50) PRIMARY KEY,
    created_at TIMESTAMPTZ,
    modified_at TIMESTAMPTZ,
    version INTEGER DEFAULT 0,
    tenant_id VARCHAR(50),
    partition_id VARCHAR(50),
    access_id VARCHAR(50),
    deleted_at TIMESTAMPTZ,

    file_id TEXT,
    action TEXT,
    source TEXT
);

CREATE INDEX IF NOT EXISTS idx_media_audit_file_id ON media_audit (file_id);
CREATE INDEX IF NOT EXISTS idx_media_audit_created_at ON media_audit (created_at);
