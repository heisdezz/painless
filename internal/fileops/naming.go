package fileops

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

var windowsReservedNames = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true,
	"COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true,
	"LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

// SanitizePathSegment cleans a single directory or file segment, stripping
// invalid filesystem characters across Windows, macOS, and Linux.
func SanitizePathSegment(segment string) string {
	// 1. Remove control characters and illegal punctuation
	var sb strings.Builder
	for _, r := range segment {
		if unicode.IsControl(r) {
			continue
		}
		switch r {
		case '<', '>', ':', '"', '/', '\\', '|', '?', '*':
			sb.WriteRune('_')
		default:
			sb.WriteRune(r)
		}
	}

	cleaned := strings.TrimSpace(sb.String())

	// 2. Remove leading/trailing dots (Windows and hidden file hygiene)
	cleaned = strings.Trim(cleaned, ".")
	cleaned = strings.TrimSpace(cleaned)

	// 3. Check for directory traversal names
	if cleaned == "" || cleaned == "." || cleaned == ".." {
		return ""
	}

	// 4. Check for Windows reserved device names
	baseUpper := strings.ToUpper(cleaned)
	if dotIdx := strings.Index(baseUpper, "."); dotIdx != -1 {
		baseUpper = baseUpper[:dotIdx]
	}
	if windowsReservedNames[baseUpper] {
		cleaned = "_" + cleaned
	}

	return cleaned
}

// SanitizeFilename cleans an entire filename preserving the extension.
func SanitizeFilename(filename string) string {
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)

	cleanBase := SanitizePathSegment(base)
	if cleanBase == "" {
		cleanBase = "media_item"
	}

	cleanExt := SanitizePathSegment(ext)
	if cleanExt != "" && !strings.HasPrefix(cleanExt, ".") {
		cleanExt = "." + cleanExt
	}

	return cleanBase + cleanExt
}

// CalculateFileSHA256 computes the hexadecimal SHA-256 hash of a file on disk.
func CalculateFileSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file for hashing %s: %w", filePath, err)
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", fmt.Errorf("failed to read file for hashing %s: %w", filePath, err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// ResolveCollision checks if targetPath exists. If it does:
// - If the file has the exact same SHA-256 as srcHash, it is identified as a duplicate (returns targetPath, isDuplicate=true).
// - If the file is different (or srcHash is empty), it appends _1, _2 up to _9999 before the extension.
func ResolveCollision(targetPath string, srcHash string) (string, bool, error) {
	_, err := os.Stat(targetPath)
	if errors.Is(err, os.ErrNotExist) {
		return targetPath, false, nil
	} else if err != nil {
		return "", false, fmt.Errorf("failed to stat target path %s: %w", targetPath, err)
	}

	// File exists; check hash if srcHash provided
	if srcHash != "" {
		existingHash, err := CalculateFileSHA256(targetPath)
		if err == nil && existingHash == srcHash {
			return targetPath, true, nil
		}
	}

	dir := filepath.Dir(targetPath)
	filename := filepath.Base(targetPath)
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)

	for i := 1; i <= 9999; i++ {
		candidateName := fmt.Sprintf("%s_%d%s", base, i, ext)
		candidatePath := filepath.Join(dir, candidateName)

		cStat, cErr := os.Stat(candidatePath)
		if errors.Is(cErr, os.ErrNotExist) {
			return candidatePath, false, nil
		}
		if cErr == nil && !cStat.IsDir() && srcHash != "" {
			cHash, err := CalculateFileSHA256(candidatePath)
			if err == nil && cHash == srcHash {
				return candidatePath, true, nil
			}
		}
	}

	return "", false, fmt.Errorf("exceeded max collision attempts (9999) for %s", targetPath)
}
