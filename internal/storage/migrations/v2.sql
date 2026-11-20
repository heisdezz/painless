-- FTS5 trigram index over media_items for the search box: case-insensitive
-- substring matches over the path and metadata text.
CREATE VIRTUAL TABLE IF NOT EXISTS media_items_fts USING fts5(path, metadata, tokenize='trigram');

-- Keep the index in sync with inserts, updates and deletes on the catalog.
CREATE TRIGGER IF NOT EXISTS media_items_ai AFTER INSERT ON media_items
BEGIN
    INSERT INTO media_items_fts(rowid, path, metadata)
        VALUES (new.id, new.current_relative_path, COALESCE(new.metadata_json, ''));
END;
CREATE TRIGGER IF NOT EXISTS media_items_au AFTER UPDATE ON media_items
BEGIN
    DELETE FROM media_items_fts WHERE rowid = old.id;
    INSERT INTO media_items_fts(rowid, path, metadata)
        VALUES (old.id, new.current_relative_path, COALESCE(new.metadata_json, ''));
END;
CREATE TRIGGER IF NOT EXISTS media_items_ad AFTER DELETE ON media_items
BEGIN
    DELETE FROM media_items_fts WHERE rowid = old.id;
END;

-- Backfill existing rows; the triggers cover changes from now on.
INSERT INTO media_items_fts(rowid, path, metadata)
    SELECT id, current_relative_path, COALESCE(metadata_json, '') FROM media_items;
