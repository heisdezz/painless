-- One row per file_hash: the library stores media by content hash. Pre-v3
-- databases may hold duplicate rows for a hash, so dedupe (keeping the
-- earliest row per hash) before enforcing the invariant.
DELETE FROM media_items
WHERE id NOT IN (SELECT MIN(id) FROM media_items GROUP BY file_hash);

DROP INDEX IF EXISTS idx_media_file_hash;
CREATE UNIQUE INDEX IF NOT EXISTS idx_media_file_hash ON media_items(file_hash);
