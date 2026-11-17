package scanner

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/h2non/filetype"
)

var mediaExtensions = map[string]string{
	// Images
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".webp": "image/webp",
	".gif":  "image/gif",
	".heic": "image/heic",
	".bmp":  "image/bmp",
	".tiff": "image/tiff",
	".tif":  "image/tiff",
	".svg":  "image/svg+xml",
	".ico":  "image/x-icon",

	// Videos
	".mp4":  "video/mp4",
	".mov":  "video/quicktime",
	".avi":  "video/x-msvideo",
	".mkv":  "video/x-matroska",
	".webm": "video/webm",
	".m4v":  "video/x-m4v",
	".flv":  "video/x-flv",
	".wmv":  "video/x-ms-wmv",
	".3gp":  "video/3gpp",

	// Audio
	".mp3":  "audio/mpeg",
	".m4a":  "audio/mp4",
	".wav":  "audio/wav",
	".flac": "audio/flac",
	".aac":  "audio/aac",
	".ogg":  "audio/ogg",
	".wma":  "audio/x-ms-wma",
}

// IsSupportedExtension returns true if the file extension is a candidate media type.
func IsSupportedExtension(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	_, exists := mediaExtensions[ext]
	return exists
}

// SniffMediaType detects the MIME type of a file using magic bytes, falling back
// to extension mapping when header bytes are generic or vendor-specific.
func SniffMediaType(filePath string) (string, bool, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	fallbackMime, isCandidate := mediaExtensions[ext]
	if !isCandidate {
		return "", false, nil
	}

	f, err := os.Open(filePath)
	if err != nil {
		return "", false, err
	}
	defer f.Close()

	// Read initial header buffer for magic byte inspection
	header := make([]byte, 8192)
	n, err := f.Read(header)
	if err != nil && err != io.EOF {
		return "", false, err
	}
	header = header[:n]

	// 1. Try filetype match
	kind, err := filetype.Match(header)
	if err == nil && kind != filetype.Unknown {
		mime := kind.MIME.Value
		if strings.HasPrefix(mime, "image/") || strings.HasPrefix(mime, "video/") || strings.HasPrefix(mime, "audio/") {
			return mime, true, nil
		}
	}

	// 2. Check if filetype recognizes general image/video/audio
	if filetype.IsImage(header) || filetype.IsVideo(header) || filetype.IsAudio(header) {
		if kind.MIME.Value != "" {
			return kind.MIME.Value, true, nil
		}
	}

	// 3. Fallback to extension-based MIME type
	if fallbackMime != "" {
		return fallbackMime, true, nil
	}

	return "", false, nil
}
