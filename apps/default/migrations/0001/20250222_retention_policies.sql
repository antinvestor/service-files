-- Retention policies table
CREATE TABLE IF NOT EXISTS retention_policies (
    id VARCHAR(50) PRIMARY KEY,
    created_at TIMESTAMPTZ,
    modified_at TIMESTAMPTZ,
    version INTEGER DEFAULT 0,
    tenant_id VARCHAR(50),
    partition_id VARCHAR(50),
    access_id VARCHAR(50),
    deleted_at TIMESTAMPTZ,

    name TEXT NOT NULL,
    description TEXT,
    retention_days INTEGER NOT NULL,
    is_default BOOLEAN DEFAULT FALSE,
    is_system BOOLEAN DEFAULT FALSE,
    owner_id TEXT,
    metadata JSONB
);

CREATE INDEX IF NOT EXISTS idx_retention_policies_owner_id ON retention_policies (owner_id);
CREATE INDEX IF NOT EXISTS idx_retention_policies_is_default ON retention_policies (is_default);

-- File retention assignments
CREATE TABLE IF NOT EXISTS file_retentions (
    id VARCHAR(50) PRIMARY KEY,
    created_at TIMESTAMPTZ,
    modified_at TIMESTAMPTZ,
    version INTEGER DEFAULT 0,
    tenant_id VARCHAR(50),
    partition_id VARCHAR(50),
    access_id VARCHAR(50),
    deleted_at TIMESTAMPTZ,

    media_id TEXT NOT NULL,
    policy_id VARCHAR(50) NOT NULL,
    applied_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at TIMESTAMPTZ,
    is_locked BOOLEAN DEFAULT FALSE,
    metadata JSONB
);

CREATE INDEX IF NOT EXISTS idx_file_retentions_media_id ON file_retentions (media_id);
CREATE INDEX IF NOT EXISTS idx_file_retentions_policy_id ON file_retentions (policy_id);
CREATE INDEX IF NOT EXISTS idx_file_retentions_expires_at ON file_retentions (expires_at);
