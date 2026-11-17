package stream

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// ErrOutsideRoot reports a request path that resolves outside the drive
// sandbox. Handlers map it to 403.
var ErrOutsideRoot = errors.New("path escapes drive root")

// ErrMissing reports a path that is inside the root but does not exist on
// disk. Handlers map it to 404.
var ErrMissing = errors.New("file not found on drive")

// ValidateRelPath performs the filesystem-independent portion of the
// sandbox check on a raw request path: it must be relative, non-empty,
// and lexically stay inside the root after cleaning. Handlers run it
// before catalog lookups so hostile paths are rejected with 403 even
// when they match no cataloged item.
func ValidateRelPath(requested string) error {
	if requested == "" {
		return fmt.Errorf("%w: empty path", ErrOutsideRoot)
	}
	if filepath.IsAbs(requested) {
		return fmt.Errorf("%w: absolute path %q", ErrOutsideRoot, requested)
	}
	cleaned := filepath.Clean(requested)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%w: %q escapes root", ErrOutsideRoot, requested)
	}
	return nil
}

// ResolvePath verifies that requested (a drive-relative path as cataloged
// in the media library) resolves to a real file inside root, and returns
// its canonical absolute path.
//
// Defense layers:
//  1. The request must be a relative path; absolute paths are rejected.
//  2. The root itself is canonicalized, following any mount symlinks.
//  3. The joined path is canonicalized with EvalSymlinks, which both
//     requires existence and defeats symlink escapes.
//  4. The canonical result must remain inside the canonical root.
func ResolvePath(root, requested string) (string, error) {
	if requested == "" {
		return "", fmt.Errorf("%w: empty path", ErrOutsideRoot)
	}
	if filepath.IsAbs(requested) {
		return "", fmt.Errorf("%w: absolute path %q", ErrOutsideRoot, requested)
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve drive root: %w", err)
	}
	canonicalRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return "", fmt.Errorf("resolve drive root: %w", err)
	}

	joined := filepath.Join(canonicalRoot, filepath.Clean(requested))
	canonical, err := filepath.EvalSymlinks(joined)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("%w: %q", ErrMissing, requested)
		}
		return "", fmt.Errorf("resolve %q: %w", requested, err)
	}

	rel, err := filepath.Rel(canonicalRoot, canonical)
	if err != nil || rel == ".." || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %q", ErrOutsideRoot, requested)
	}
	return canonical, nil
}
