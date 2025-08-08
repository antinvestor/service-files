
-- Recreate with 'simple' configuration and handle empty jsonb_to_tsv properly
ALTER TABLE media_metadata
    ADD COLUMN search_properties tsvector GENERATED ALWAYS AS (
        jsonb_to_tsv(COALESCE(properties, '{}'::jsonb))
        ) STORED;

-- Recreate the GIN index
CREATE INDEX idx_media_metadata_search_properties ON media_metadata USING GIN (search_properties);
