-- Persist where each media object lives. Rows written before this keep an
-- empty object_key/bucket_name and are resolved through the legacy
-- hash-derived key at read time (storage.ResolveObjectLocation).
ALTER TABLE media_metadata ADD COLUMN IF NOT EXISTS object_key TEXT;

-- DeleteContent counts the other live rows sharing an object before it
-- removes the object from the bucket.
CREATE INDEX IF NOT EXISTS idx_media_metadata_bucket_object_key
    ON media_metadata (bucket_name, object_key);
