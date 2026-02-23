-- Multipart upload tracking table
CREATE TABLE IF NOT EXISTS multipart_uploads (
    id VARCHAR(50) PRIMARY KEY,
    created_at TIMESTAMPTZ,
    modified_at TIMESTAMPTZ,
    version INTEGER DEFAULT 0,
    tenant_id VARCHAR(50),
    partition_id VARCHAR(50),
    access_id VARCHAR(50),
    deleted_at TIMESTAMPTZ,

    owner_id TEXT NOT NULL,
    media_id TEXT NOT NULL,
    upload_name TEXT,
    content_type TEXT,
    total_size BIGINT,
    part_size BIGINT,
    part_count INTEGER,
    uploaded_parts INTEGER DEFAULT 0,
    upload_state VARCHAR(20) DEFAULT 'pending',
    expires_at TIMESTAMPTZ,
    metadata JSONB
);

CREATE INDEX IF NOT EXISTS idx_multipart_uploads_owner_id ON multipart_uploads (owner_id);
CREATE INDEX IF NOT EXISTS idx_multipart_uploads_media_id ON multipart_uploads (media_id);
CREATE INDEX IF NOT EXISTS idx_multipart_uploads_expires_at ON multipart_uploads (expires_at);

-- Individual upload parts table
CREATE TABLE IF NOT EXISTS multipart_upload_parts (
    id VARCHAR(50) PRIMARY KEY,
    created_at TIMESTAMPTZ,
    modified_at TIMESTAMPTZ,
    version INTEGER DEFAULT 0,
    tenant_id VARCHAR(50),
    partition_id VARCHAR(50),
    access_id VARCHAR(50),
    deleted_at TIMESTAMPTZ,

    upload_id VARCHAR(50) NOT NULL,
    part_number INTEGER NOT NULL,
    etag TEXT,
    size BIGINT,
    content_hash TEXT,
    storage_path TEXT,
    is_uploaded BOOLEAN DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS idx_multipart_upload_parts_upload_id ON multipart_upload_parts (upload_id);
CREATE INDEX IF NOT EXISTS idx_multipart_upload_parts_upload_part ON multipart_upload_parts (upload_id, part_number);
