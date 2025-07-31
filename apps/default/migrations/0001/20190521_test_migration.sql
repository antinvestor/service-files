

CREATE INDEX media_search_idx ON media_metadata
    USING bm25 (id, parent_id, owner_id, name, ext, public, hash, properties, created_at)
    WITH (
    key_field='id',
    json_fields = '{ "properties": {"fast": true}}'
    );
