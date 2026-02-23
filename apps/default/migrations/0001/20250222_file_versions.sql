-- File versions table for version history
CREATE TABLE IF NOT EXISTS file_versions (
    id VARCHAR(50) PRIMARY KEY,
    created_at TIMESTAMPTZ,
    modified_at TIMESTAMPTZ,
    version INTEGER DEFAULT 0,
    tenant_id VARCHAR(50),
    partition_id VARCHAR(50),
    access_id VARCHAR(50),
    deleted_at TIMESTAMPTZ,

    media_id TEXT NOT NULL,
    version_number INTEGER NOT NULL,
    content_hash TEXT NOT NULL,
    file_size BIGINT,
    upload_name TEXT,
    content_type TEXT,
    storage_path TEXT,
    created_by TEXT,
    restore_from_version INTEGER,
    metadata JSONB
);

CREATE INDEX IF NOT EXISTS idx_file_versions_media_id ON file_versions (media_id);
CREATE INDEX IF NOT EXISTS idx_file_versions_media_version ON file_versions (media_id, version_number);
CREATE INDEX IF NOT EXISTS idx_file_versions_created_at ON file_versions (created_at);
