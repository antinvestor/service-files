-- Persist where each media object lives. Rows written before this keep an
-- empty object_key/bucket_name and are resolved through the legacy
-- hash-derived key at read time (storage.ResolveObjectLocation).
ALTER TABLE media_metadata ADD COLUMN IF NOT EXISTS object_key TEXT;

-- Deleting media counts the other live rows that share its object before
-- removing the object from the bucket. One index serves rows with a recorded
-- key, and a partial index serves legacy rows, matched by hash and visibility.
--
-- CONCURRENTLY is not used: the Frame migrator applies each file inside a
-- transaction, where CREATE INDEX CONCURRENTLY is not allowed. A plain build
-- takes a SHARE lock that blocks writes to media_metadata (reads continue)
-- for as long as the build runs. On a large table, create these indexes by
-- hand with CONCURRENTLY before deploying; IF NOT EXISTS then skips them here.
CREATE INDEX IF NOT EXISTS idx_media_metadata_bucket_object_key
    ON media_metadata (bucket_name, object_key);

CREATE INDEX IF NOT EXISTS idx_media_metadata_legacy_object
    ON media_metadata (hash, public)
    WHERE COALESCE(object_key, '') = '';
