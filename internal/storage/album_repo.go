package storage

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"painless/internal/domain"
)

// AlbumRepo manages persistence operations for albums.
type AlbumRepo struct {
	db *DB
}

// NewAlbumRepo creates an AlbumRepo.
func NewAlbumRepo(db *DB) *AlbumRepo {
	return &AlbumRepo{db: db}
}

// Create creates a new album record in the database.
func (r *AlbumRepo) Create(ctx context.Context, input domain.CreateAlbumInput) (*domain.Album, error) {
	if !domain.ValidateAlbumName(input.Name) {
		return nil, fmt.Errorf("invalid album name: %q", input.Name)
	}

	relPath := filepath.ToSlash(filepath.Join("albums", input.Name))

	var id int64
	err := r.db.WithTx(ctx, func(tx *sql.Tx) error {
		query := `
			INSERT INTO albums (name, relative_path, description, created_at)
			VALUES (?, ?, ?, ?)
		`
		now := time.Now().UTC()
		res, err := tx.ExecContext(ctx, query, input.Name, relPath, input.Description, now)
		if err != nil {
			return err
		}
		id, err = res.LastInsertId()
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to insert album: %w", err)
	}

	return &domain.Album{
		ID:           id,
		Name:         input.Name,
		RelativePath: relPath,
		Description:  input.Description,
		CreatedAt:    time.Now().UTC(),
	}, nil
}

// GetByID returns an album by its database ID.
func (r *AlbumRepo) GetByID(ctx context.Context, id int64) (*domain.Album, error) {
	query := `
		SELECT id, name, relative_path, description, created_at
		FROM albums
		WHERE id = ?
	`
	var album domain.Album
	var desc sql.NullString
	var createdAt string

	err := r.db.Raw().QueryRowContext(ctx, query, id).Scan(
		&album.ID,
		&album.Name,
		&album.RelativePath,
		&desc,
		&createdAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query album %d: %w", id, err)
	}

	if desc.Valid {
		album.Description = &desc.String
	}
	album.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)

	return &album, nil
}

// GetByName returns an album by its unique name.
func (r *AlbumRepo) GetByName(ctx context.Context, name string) (*domain.Album, error) {
	query := `
		SELECT id, name, relative_path, description, created_at
		FROM albums
		WHERE name = ?
	`
	var album domain.Album
	var desc sql.NullString
	var createdAt string

	err := r.db.Raw().QueryRowContext(ctx, query, name).Scan(
		&album.ID,
		&album.Name,
		&album.RelativePath,
		&desc,
		&createdAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query album by name %q: %w", name, err)
	}

	if desc.Valid {
		album.Description = &desc.String
	}
	album.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)

	return &album, nil
}

// List returns all albums with member counts, total size in bytes, and preview item.
func (r *AlbumRepo) List(ctx context.Context) ([]domain.AlbumSummary, error) {
	query := `
		SELECT
			a.id, a.name, a.relative_path, a.description, a.created_at,
			COUNT(m.id) as media_count,
			COALESCE(SUM(m.file_size), 0) as total_bytes,
			p.id as prev_id, p.file_hash as prev_hash, p.current_relative_path as prev_path, p.mime_type as prev_mime
		FROM albums a
		LEFT JOIN media_items m ON m.album_id = a.id
		LEFT JOIN media_items p ON p.id = (
			SELECT id FROM media_items WHERE album_id = a.id ORDER BY id ASC LIMIT 1
		)
		GROUP BY a.id
		ORDER BY a.name COLLATE NOCASE ASC
	`

	rows, err := r.db.Raw().QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list albums: %w", err)
	}
	defer rows.Close()

	var summaries []domain.AlbumSummary
	for rows.Next() {
		var s domain.AlbumSummary
		var desc sql.NullString
		var createdAt string
		var prevID sql.NullInt64
		var prevHash, prevPath, prevMime sql.NullString

		err := rows.Scan(
			&s.ID,
			&s.Name,
			&s.RelativePath,
			&desc,
			&createdAt,
			&s.MediaCount,
			&s.TotalBytes,
			&prevID,
			&prevHash,
			&prevPath,
			&prevMime,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan album row: %w", err)
		}

		if desc.Valid {
			s.Description = &desc.String
		}
		s.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)

		if prevID.Valid {
			s.PreviewItem = &domain.MediaItem{
				ID:                  prevID.Int64,
				FileHash:            prevHash.String,
				CurrentRelativePath: prevPath.String,
				MimeType:            prevMime.String,
			}
		}

		summaries = append(summaries, s)
	}

	return summaries, rows.Err()
}

// Update renames or edits an album description.
func (r *AlbumRepo) Update(ctx context.Context, input domain.EditAlbumInput) error {
	if !domain.ValidateAlbumName(input.NewName) {
		return fmt.Errorf("invalid new album name: %q", input.NewName)
	}

	newRelPath := filepath.ToSlash(filepath.Join("albums", input.NewName))

	return r.db.WithTx(ctx, func(tx *sql.Tx) error {
		query := `
			UPDATE albums
			SET name = ?, relative_path = ?, description = ?
			WHERE id = ?
		`
		res, err := tx.ExecContext(ctx, query, input.NewName, newRelPath, input.Description, input.ID)
		if err != nil {
			return err
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			return fmt.Errorf("album with id %d not found", input.ID)
		}
		return nil
	})
}

// Delete removes an album record. If permanent is false, associated items are unassigned (album_id = NULL).
// If permanent is true, associated media items are deleted from the database.
func (r *AlbumRepo) Delete(ctx context.Context, albumID int64, permanent bool) (int64, error) {
	var affectedItems int64

	err := r.db.WithTx(ctx, func(tx *sql.Tx) error {
		if permanent {
			res, err := tx.ExecContext(ctx, "DELETE FROM media_items WHERE album_id = ?", albumID)
			if err != nil {
				return err
			}
			affectedItems, _ = res.RowsAffected()
		} else {
			res, err := tx.ExecContext(ctx, "UPDATE media_items SET album_id = NULL WHERE album_id = ?", albumID)
			if err != nil {
				return err
			}
			affectedItems, _ = res.RowsAffected()
		}

		_, err := tx.ExecContext(ctx, "DELETE FROM albums WHERE id = ?", albumID)
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("failed to delete album %d: %w", albumID, err)
	}

	return affectedItems, nil
}

// Merge reassigns all media items from source albums into targetAlbumID, then removes source albums.
func (r *AlbumRepo) Merge(ctx context.Context, targetAlbumID int64, sourceAlbumIDs []int64) (int64, error) {
	if len(sourceAlbumIDs) == 0 {
		return 0, nil
	}

	var movedCount int64
	err := r.db.WithTx(ctx, func(tx *sql.Tx) error {
		// Verify target exists
		var targetExists int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM albums WHERE id = ?", targetAlbumID).Scan(&targetExists); err != nil || targetExists == 0 {
			return fmt.Errorf("target album %d does not exist", targetAlbumID)
		}

		// Reassign items
		placeholders := strings.Repeat("?,", len(sourceAlbumIDs))
		placeholders = placeholders[:len(placeholders)-1]

		args := make([]any, 0, len(sourceAlbumIDs)+1)
		args = append(args, targetAlbumID)
		for _, id := range sourceAlbumIDs {
			args = append(args, id)
		}

		updateQuery := fmt.Sprintf("UPDATE media_items SET album_id = ? WHERE album_id IN (%s)", placeholders)
		res, err := tx.ExecContext(ctx, updateQuery, args...)
		if err != nil {
			return err
		}
		movedCount, _ = res.RowsAffected()

		// Delete source albums
		delArgs := make([]any, 0, len(sourceAlbumIDs))
		for _, id := range sourceAlbumIDs {
			delArgs = append(delArgs, id)
		}
		delQuery := fmt.Sprintf("DELETE FROM albums WHERE id IN (%s)", placeholders)
		if _, err := tx.ExecContext(ctx, delQuery, delArgs...); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("failed to merge albums: %w", err)
	}

	return movedCount, nil
}
