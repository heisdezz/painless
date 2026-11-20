//go:build linux

package fileops

import (
	"golang.org/x/sys/unix"
)

// renameNoReplace performs an atomic rename that fails with os.ErrExist
// when dst already exists, so no-replace moves can't clobber a racing
// writer.
func renameNoReplace(src, dst string) error {
	return unix.Renameat2(unix.AT_FDCWD, src, unix.AT_FDCWD, dst, unix.RENAME_NOREPLACE)
}
