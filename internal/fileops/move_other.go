//go:build !linux

package fileops

import (
	"errors"
	"os"
)

// errNoReplaceFallback tells MoveFileNoReplace to use the exclusive-create
// copy path: this platform has no atomic no-replace rename.
var errNoReplaceFallback = errors.New("no atomic no-replace rename on this platform")

// renameNoReplace fails with os.ErrExist when dst already exists; otherwise
// it returns errNoReplaceFallback so the move falls through to the
// race-safe exclusive-create copy path.
func renameNoReplace(src, dst string) error {
	if _, err := os.Stat(dst); err == nil {
		return os.ErrExist
	} else if !os.IsNotExist(err) {
		return err
	}
	return errNoReplaceFallback
}
