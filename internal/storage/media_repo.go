package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"painless/internal/domain"
)

// MediaRepo handles querying and persisting media item records in SQLite.
type MediaRepo struct {
	db *DB
}

// NewMediaRepo creates a new MediaRepo.
func NewMediaRepo(db *DB) *MediaRepo {
	return &MediaRepo{db: db}
}

// InsertBatch inserts multiple media records in a single transactional batch.
func (r *MediaRepo) InsertBatch(ctx context.Context, items []domain.MediaItem) (int, error) {
	return r.insertBatch(ctx, items, nil)
}

// InsertScanBatch returns the committed items with their database IDs so they
// can be queued for review without a second lookup.
func (r *MediaRepo) InsertScanBatch(ctx context.Context, items []domain.MediaItem) ([]domain.MediaItem, error) {
	inserted := make([]domain.MediaItem, 0, len(items))
	_, err := r.insertBatch(ctx, items, func(item domain.MediaItem) {
		inserted = append(inserted, item)
	})
	if err != nil {
		return nil, err
	}
	return inserted, nil
}

// isUniqueViolation reports whether err is the driver's unique-constraint
// violation (modernc returns the extended SQLITE_CONSTRAINT_UNIQUE code).
func isUniqueViolation(err error) bool {
	var serr *sqlite.Error
	return errors.As(err, &serr) && serr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE
}

func (r *MediaRepo) insertBatch(ctx context.Context, items []domain.MediaItem, collect func(domain.MediaItem)) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}

	var inserted int
	err := r.db.WithTx(ctx, func(tx *sql.Tx) error {
		query := `
			INSERT INTO media_items (
				file_hash, original_relative_path, current_relative_path,
				file_size, mime_type, duration_seconds, metadata_json,
				album_id, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`
		stmt, err := tx.PrepareContext(ctx, query)
		if err != nil {
			return err
		}
		defer stmt.Close()

		for _, item := range items {
			createdAt := item.CreatedAt
			if createdAt.IsZero() {
				createdAt = time.Now().UTC()
			}

			result, err := stmt.ExecContext(
				ctx,
				item.FileHash,
				item.OriginalRelativePath,
				item.CurrentRelativePath,
				item.FileSize,
				item.MimeType,
				item.DurationSeconds,
				item.MetadataJSON,
				item.AlbumID,
				createdAt.Format("2006-01-02 15:04:05"),
			)
			if err != nil {
				if isUniqueViolation(err) {
					// Library dedup invariant: one row per file_hash. A
					// concurrent insert (or an upload of identical
					// content) wins; this item is a duplicate, not a
					// data-loss case.
					continue
				}
				return err
			}
			if collect != nil {
				item.ID, err = result.LastInsertId()
				if err != nil {
					return err
				}
				collect(item)
			}
			inserted++
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("failed to insert media items batch: %w", err)
	}

	return inserted, nil
}

// GetByID returns a single media item by its ID.
func (r *MediaRepo) GetByID(ctx context.Context, id int64) (*domain.MediaItem, error) {
	query := `
		SELECT
			id, file_hash, original_relative_path, current_relative_path,
			file_size, mime_type, duration_seconds, metadata_json,
			album_id, created_at
		FROM media_items
		WHERE id = ?
	`
	item, err := scanMediaItem(r.db.Raw().QueryRowContext(ctx, query, id))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get media item %d: %w", id, err)
	}
	return item, nil
}

// GetByPath returns the item cataloged at an exact drive-relative path.
func (r *MediaRepo) GetByPath(ctx context.Context, relPath string) (*domain.MediaItem, error) {
	const query = `
		SELECT id, file_hash, original_relative_path, current_relative_path,
		       file_size, mime_type, duration_seconds, metadata_json, album_id, created_at
		FROM media_items WHERE current_relative_path = ? LIMIT 1
	`
	item, err := scanMediaItem(r.db.Raw().QueryRowContext(ctx, query, relPath))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get media item by path %q: %w", relPath, err)
	}
	return item, nil
}

// GetItemWithNavigation retrieves a media item along with adjacent previous and next item IDs.
func (r *MediaRepo) GetItemWithNavigation(ctx context.Context, id int64, albumID *int64) (*domain.MediaItem, *int64, *int64, error) {
	item, err := r.GetByID(ctx, id)
	if err != nil || item == nil {
		return item, nil, nil, err
	}

	var prevID, nextID *int64

	// Determine navigation queries based on album scope
	var prevQuery, nextQuery string
	var argsPrev, argsNext []any

	if albumID != nil {
		prevQuery = "SELECT id FROM media_items WHERE album_id = ? AND id < ? ORDER BY id DESC LIMIT 1"
		argsPrev = []any{*albumID, id}
		nextQuery = "SELECT id FROM media_items WHERE album_id = ? AND id > ? ORDER BY id ASC LIMIT 1"
		argsNext = []any{*albumID, id}
	} else {
		prevQuery = "SELECT id FROM media_items WHERE id < ? ORDER BY id DESC LIMIT 1"
		argsPrev = []any{id}
		nextQuery = "SELECT id FROM media_items WHERE id > ? ORDER BY id ASC LIMIT 1"
		argsNext = []any{id}
	}

	var pID int64
	if err := r.db.Raw().QueryRowContext(ctx, prevQuery, argsPrev...).Scan(&pID); err == nil {
		prevID = &pID
	}

	var nID int64
	if err := r.db.Raw().QueryRowContext(ctx, nextQuery, argsNext...).Scan(&nID); err == nil {
		nextID = &nID
	}

	return item, prevID, nextID, nil
}

// GetByHash retrieves a media item by its unique SHA-256 hash.
func (r *MediaRepo) GetByHash(ctx context.Context, fileHash string) (*domain.MediaItem, error) {
	query := `
		SELECT
			id, file_hash, original_relative_path, current_relative_path,
			file_size, mime_type, duration_seconds, metadata_json,
			album_id, created_at
		FROM media_items
		WHERE file_hash = ?
		LIMIT 1
	`
	item, err := scanMediaItem(r.db.Raw().QueryRowContext(ctx, query, fileHash))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get media item by hash %q: %w", fileHash, err)
	}
	return item, nil
}

// Count returns the exact number of indexed media items in this library.
func (r *MediaRepo) Count(ctx context.Context) (int64, error) {
	var total int64
	if err := r.db.Raw().QueryRowContext(ctx, "SELECT COUNT(*) FROM media_items").Scan(&total); err != nil {
		return 0, fmt.Errorf("failed to count media items: %w", err)
	}
	return total, nil
}

// List queries paginated media items according to filtering and sorting parameters.

// minFTSSearch is the shortest search string routed to the FTS index. The
// trigram tokenizer only matches 3+ character terms, so shorter searches
// keep the LIKE fallback.
const minFTSSearch = 3

// ftsMatchQuery quotes a user search string as one FTS5 phrase for the
// trigram index: a case-insensitive substring match over the indexed path
// and metadata text — the same semantics the previous LIKE clause had.
// Embedded double quotes are escaped.
func ftsMatchQuery(search string) string {
	return `"` + strings.ReplaceAll(search, `"`, `""`) + `"`
}

func (r *MediaRepo) List(ctx context.Context, params domain.MediaListParams) (*domain.MediaListPage, error) {
	var whereClauses []string
	var args []any

	if params.AlbumID != nil {
		whereClauses = append(whereClauses, "album_id = ?")
		args = append(args, *params.AlbumID)
	}

	if params.Search != "" {
		if len(params.Search) >= minFTSSearch {
			whereClauses = append(whereClauses,
				"rowid IN (SELECT rowid FROM media_items_fts WHERE media_items_fts MATCH ?)")
			args = append(args, ftsMatchQuery(params.Search))
		} else {
			// Short searches keep the LIKE fallback (the trigram index only
			// matches 3+ character terms).
			whereClauses = append(whereClauses, "(current_relative_path LIKE ? OR metadata_json LIKE ?)")
			pattern := "%" + params.Search + "%"
			args = append(args, pattern, pattern)
		}
	}

	switch params.Filter {
	case domain.MediaFilterImages:
		whereClauses = append(whereClauses, "mime_type LIKE 'image/%'")
	case domain.MediaFilterVideos:
		whereClauses = append(whereClauses, "mime_type LIKE 'video/%'")
	case domain.MediaFilterAudio:
		whereClauses = append(whereClauses, "mime_type LIKE 'audio/%'")
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	// 1. Get total count
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM media_items %s", whereSQL)
	var total int
	if err := r.db.Raw().QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to count media items: %w", err)
	}

	// 2. Determine sort order
	sortColumn := "created_at"
	switch params.SortBy {
	case domain.SortByName:
		sortColumn = "current_relative_path COLLATE NOCASE"
	case domain.SortBySize:
		sortColumn = "file_size"
	case domain.SortByDate:
		sortColumn = "created_at"
	case domain.SortByAdded:
		sortColumn = "id"
	}

	direction := "DESC"
	if strings.ToLower(string(params.SortOrder)) == "asc" {
		direction = "ASC"
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := params.Offset
	if offset < 0 {
		offset = 0
	}

	// 3. Query rows
	dataQuery := fmt.Sprintf(`
		SELECT
			id, file_hash, original_relative_path, current_relative_path,
			file_size, mime_type, duration_seconds, metadata_json,
			album_id, created_at
		FROM media_items
		%s
		ORDER BY %s %s, id %s
		LIMIT ? OFFSET ?
	`, whereSQL, sortColumn, direction, direction)

	queryArgs := append(args, limit, offset)
	rows, err := r.db.Raw().QueryContext(ctx, dataQuery, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to query media items: %w", err)
	}
	defer rows.Close()

	var items []domain.MediaItem
	for rows.Next() {
		item, err := scanMediaItem(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan media item: %w", err)
		}
		items = append(items, *item)
	}

	return &domain.MediaListPage{
		Items:  items,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, rows.Err()
}

// MoveToAlbum assigns multiple media items to a target album.
func (r *MediaRepo) MoveToAlbum(ctx context.Context, targetAlbumID int64, mediaIDs []int64) (int64, error) {
	if len(mediaIDs) == 0 {
		return 0, nil
	}

	placeholders := strings.Repeat("?,", len(mediaIDs))
	placeholders = placeholders[:len(placeholders)-1]

	args := make([]any, 0, len(mediaIDs)+1)
	args = append(args, targetAlbumID)
	for _, id := range mediaIDs {
		args = append(args, id)
	}

	query := fmt.Sprintf("UPDATE media_items SET album_id = ? WHERE id IN (%s)", placeholders)
	res, err := r.db.Raw().ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to move media items to album %d: %w", targetAlbumID, err)
	}

	return res.RowsAffected()
}

// RemoveFromAlbum clears the album assignment for specified media items.
func (r *MediaRepo) RemoveFromAlbum(ctx context.Context, mediaIDs []int64) (int64, error) {
	if len(mediaIDs) == 0 {
		return 0, nil
	}

	placeholders := strings.Repeat("?,", len(mediaIDs))
	placeholders = placeholders[:len(placeholders)-1]

	args := make([]any, 0, len(mediaIDs))
	for _, id := range mediaIDs {
		args = append(args, id)
	}

	query := fmt.Sprintf("UPDATE media_items SET album_id = NULL WHERE id IN (%s)", placeholders)
	res, err := r.db.Raw().ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to remove media items from album: %w", err)
	}

	return res.RowsAffected()
}

// DeleteBatch removes multiple media items by ID.
func (r *MediaRepo) DeleteBatch(ctx context.Context, mediaIDs []int64) (int64, error) {
	if len(mediaIDs) == 0 {
		return 0, nil
	}

	placeholders := strings.Repeat("?,", len(mediaIDs))
	placeholders = placeholders[:len(placeholders)-1]

	args := make([]any, 0, len(mediaIDs))
	for _, id := range mediaIDs {
		args = append(args, id)
	}

	query := fmt.Sprintf("DELETE FROM media_items WHERE id IN (%s)", placeholders)
	res, err := r.db.Raw().ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to delete media items batch: %w", err)
	}

	return res.RowsAffected()
}

// UpdatePath updates the current relative path of a media item after physical relocation.
func (r *MediaRepo) UpdatePath(ctx context.Context, id int64, newPath string) error {
	query := "UPDATE media_items SET current_relative_path = ? WHERE id = ?"
	res, err := r.db.Raw().ExecContext(ctx, query, newPath, id)
	if err != nil {
		return fmt.Errorf("failed to update path for media item %d: %w", id, err)
	}
	affected, err := res.RowsAffected()
	if err != nil || affected == 0 {
		return fmt.Errorf("media item %d not found", id)
	}
	return nil
}

// ScannerRow abstracts sql.Row and sql.Rows for scanMediaItem.
type ScannerRow interface {
	Scan(dest ...any) error
}

func scanMediaItem(row ScannerRow) (*domain.MediaItem, error) {
	var item domain.MediaItem
	var duration sql.NullInt64
	var metadata sql.NullString
	var albumID sql.NullInt64
	var createdAt string

	err := row.Scan(
		&item.ID,
		&item.FileHash,
		&item.OriginalRelativePath,
		&item.CurrentRelativePath,
		&item.FileSize,
		&item.MimeType,
		&duration,
		&metadata,
		&albumID,
		&createdAt,
	)
	if err != nil {
		return nil, err
	}

	if duration.Valid {
		d := int(duration.Int64)
		item.DurationSeconds = &d
	}
	if metadata.Valid {
		item.MetadataJSON = metadata.String
	}
	if albumID.Valid {
		item.AlbumID = &albumID.Int64
	}
	item.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)

	return &item, nil
}
