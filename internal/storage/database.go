package storage

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	_ "modernc.org/sqlite"
)

// LibraryDBFilename is the standardized SQLite database file stored at the root of a media drive.
const LibraryDBFilename = ".media_library.db"

// DB wraps the underlying *sql.DB connection pool with media library capabilities.
type DB struct {
	db       *sql.DB
	path     string
	readOnly bool
	mu       sync.RWMutex
}

// OpenOptions configures connection behavior.
type OpenOptions struct {
	ReadOnly bool
}

// Open initializes or connects to a SQLite media library database at the specified drive root.
func Open(driveRoot string, opts OpenOptions) (*DB, error) {
	if driveRoot == "" {
		return nil, fmt.Errorf("driveRoot path cannot be empty")
	}

	dbPath := filepath.Join(driveRoot, LibraryDBFilename)

	// Ensure parent directory exists
	if !opts.ReadOnly {
		if err := os.MkdirAll(driveRoot, 0755); err != nil {
			return nil, fmt.Errorf("failed to create drive directory %s: %w", driveRoot, err)
		}
	}

	connStr := buildDSN(dbPath, opts.ReadOnly)
	sqlDB, err := sql.Open("sqlite", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite connection: %w", err)
	}

	// Optimize connection pooling for WAL mode
	maxConns := max(4, runtime.NumCPU())
	sqlDB.SetMaxOpenConns(maxConns)
	sqlDB.SetMaxIdleConns(2)

	// Ping database
	if err := sqlDB.Ping(); err != nil {
		// If write attempt fails due to read-only filesystem, attempt read-only fallback
		if !opts.ReadOnly && isReadOnlyError(err) {
			_ = sqlDB.Close()
			return Open(driveRoot, OpenOptions{ReadOnly: true})
		}
		_ = sqlDB.Close()
		return nil, fmt.Errorf("failed to ping sqlite database at %s: %w", dbPath, err)
	}

	// Apply pragmatic configuration
	if !opts.ReadOnly {
		pragmas := []string{
			"PRAGMA journal_mode = WAL;",
			"PRAGMA synchronous = NORMAL;",
			"PRAGMA foreign_keys = ON;",
			"PRAGMA busy_timeout = 5000;",
		}
		for _, pragma := range pragmas {
			if _, err := sqlDB.Exec(pragma); err != nil {
				_ = sqlDB.Close()
				return nil, fmt.Errorf("failed to execute %s: %w", pragma, err)
			}
		}

		// Run embedded schema migrations
		if err := RunMigrations(sqlDB); err != nil {
			_ = sqlDB.Close()
			return nil, fmt.Errorf("failed to apply migrations: %w", err)
		}
	}

	return &DB{
		db:       sqlDB,
		path:     dbPath,
		readOnly: opts.ReadOnly,
	}, nil
}

// OpenMemory creates an in-memory SQLite database instance (primarily for unit testing).
func OpenMemory() (*DB, error) {
	sqlDB, err := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("failed to open in-memory sqlite: %w", err)
	}
	sqlDB.SetMaxOpenConns(1) // Single connection for memory database to retain schema

	if err := RunMigrations(sqlDB); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("failed to run migrations on memory db: %w", err)
	}

	return &DB{
		db:       sqlDB,
		path:     ":memory:",
		readOnly: false,
	}, nil
}

// Raw returns the underlying *sql.DB.
func (d *DB) Raw() *sql.DB {
	return d.db
}

// Path returns the absolute path to the database file.
func (d *DB) Path() string {
	return d.path
}

// IsReadOnly returns true if the database was opened in read-only mode.
func (d *DB) IsReadOnly() bool {
	return d.readOnly
}

// Close closes the database connection.
func (d *DB) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

// WithTx executes the provided function within a database transaction.
func (d *DB) WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	if d.readOnly {
		return fmt.Errorf("cannot execute write transaction: database is in read-only mode")
	}

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func buildDSN(dbPath string, readOnly bool) string {
	v := url.Values{}
	v.Set("_pragma", "busy_timeout(5000)")
	if readOnly {
		v.Set("mode", "ro")
	} else {
		v.Add("_pragma", "journal_mode(WAL)")
		v.Add("_pragma", "synchronous(NORMAL)")
		v.Add("_pragma", "foreign_keys(ON)")
	}
	return fmt.Sprintf("file:%s?%s", filepath.ToSlash(dbPath), v.Encode())
}

func isReadOnlyError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "readonly") || strings.Contains(msg, "permission denied")
}
