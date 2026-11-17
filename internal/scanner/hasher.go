package scanner

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

const (
	// FastHashThreshold is the file size (50 MB) above which fast sparse hashing is supported.
	FastHashThreshold = 50 * 1024 * 1024
	// SparseSampleSize is the chunk size sampled at start, middle, and end.
	SparseSampleSize = 64 * 1024
)

// ComputeFullSHA256 calculates the standard full SHA-256 hex digest of a file.
func ComputeFullSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed opening file for hash %s: %w", filePath, err)
	}
	defer f.Close()

	hasher := sha256.New()
	buf := make([]byte, 64*1024)
	if _, err := io.CopyBuffer(hasher, f, buf); err != nil {
		return "", fmt.Errorf("failed reading file for hash %s: %w", filePath, err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// ComputeFastHash calculates a sparse hash (header, middle, footer, plus total byte length)
// for fast duplicate candidate screening on large files.
func ComputeFastHash(filePath string, fileSize int64) (string, error) {
	if fileSize <= FastHashThreshold {
		return ComputeFullSHA256(filePath)
	}

	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed opening file for sparse hash %s: %w", filePath, err)
	}
	defer f.Close()

	hasher := sha256.New()

	// 1. Write file length as big-endian int64
	var sizeBytes [8]byte
	binary.BigEndian.PutUint64(sizeBytes[:], uint64(fileSize))
	hasher.Write(sizeBytes[:])

	sample := make([]byte, SparseSampleSize)

	// 2. Read first chunk
	n, _ := io.ReadFull(f, sample)
	hasher.Write(sample[:n])

	// 3. Read middle chunk
	midOffset := fileSize / 2
	if _, err := f.Seek(midOffset, io.SeekStart); err == nil {
		n, _ = io.ReadFull(f, sample)
		hasher.Write(sample[:n])
	}

	// 4. Read last chunk
	lastOffset := fileSize - int64(SparseSampleSize)
	if lastOffset > 0 {
		if _, err := f.Seek(lastOffset, io.SeekStart); err == nil {
			n, _ = io.ReadFull(f, sample)
			hasher.Write(sample[:n])
		}
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}
