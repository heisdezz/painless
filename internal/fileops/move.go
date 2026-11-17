package fileops

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// MoveFile moves a file from src to dst.
// It first attempts an atomic os.Rename. If that fails (such as across different mount points
// or filesystem boundaries), it safely falls back to a buffered copy with SHA-256 integrity
// verification before removing the source file.
func MoveFile(src, dst string) error {
	cleanSrc := filepath.Clean(src)
	cleanDst := filepath.Clean(dst)

	if cleanSrc == cleanDst {
		return nil
	}

	srcInfo, err := os.Stat(cleanSrc)
	if err != nil {
		return fmt.Errorf("source file does not exist %s: %w", cleanSrc, err)
	}
	if srcInfo.IsDir() {
		return fmt.Errorf("source is a directory, not a file: %s", cleanSrc)
	}

	// Ensure destination directory exists
	dstDir := filepath.Dir(cleanDst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", dstDir, err)
	}

	// 1. Attempt fast atomic rename (works within same partition/filesystem)
	renameErr := os.Rename(cleanSrc, cleanDst)
	if renameErr == nil {
		return nil
	}

	// 2. Fallback: Buffered copy across filesystem boundaries with verification
	return copyAndVerifyMove(cleanSrc, cleanDst, srcInfo)
}

func copyAndVerifyMove(src, dst string, srcInfo os.FileInfo) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source for copy %s: %w", src, err)
	}
	defer srcFile.Close()

	// Create destination file exclusively to avoid accidental overwrites
	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_EXCL, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("failed to open destination for write %s: %w", dst, err)
	}

	srcHasher := sha256.New()
	multiWriter := io.MultiWriter(dstFile, srcHasher)

	// Stream copy in 64KB chunks
	buf := make([]byte, 64*1024)
	written, copyErr := io.CopyBuffer(multiWriter, srcFile, buf)
	if copyErr != nil {
		dstFile.Close()
		_ = os.Remove(dst)
		return fmt.Errorf("failed copying data from %s to %s: %w", src, dst, copyErr)
	}

	// Flush to disk
	if syncErr := dstFile.Sync(); syncErr != nil {
		dstFile.Close()
		_ = os.Remove(dst)
		return fmt.Errorf("failed syncing destination file %s: %w", dst, syncErr)
	}

	if closeErr := dstFile.Close(); closeErr != nil {
		_ = os.Remove(dst)
		return fmt.Errorf("failed closing destination file %s: %w", dst, closeErr)
	}

	// Verify written byte count
	if written != srcInfo.Size() {
		_ = os.Remove(dst)
		return fmt.Errorf("size mismatch: copied %d bytes, expected %d", written, srcInfo.Size())
	}

	// Read back destination to verify checksum matches written stream
	srcChecksum := hex.EncodeToString(srcHasher.Sum(nil))
	dstChecksum, hashErr := CalculateFileSHA256(dst)
	if hashErr != nil || dstChecksum != srcChecksum {
		_ = os.Remove(dst)
		return fmt.Errorf("checksum verification failed: src=%s, dst=%s (err: %v)", srcChecksum, dstChecksum, hashErr)
	}

	// Verification succeeded; remove source
	srcFile.Close()
	if remErr := os.Remove(src); remErr != nil {
		return fmt.Errorf("file copied to %s, but failed removing original %s: %w", dst, src, remErr)
	}

	return nil
}
