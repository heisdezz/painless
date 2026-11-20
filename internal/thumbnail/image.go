package thumbnail

import (
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"os"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// ErrOversizedImage reports an image too large to decode fully for
// thumbnail generation.
var ErrOversizedImage = errors.New("image exceeds the decode pixel limit")

// maxDecodePixels bounds the decode allocation before generating a
// thumbnail; past the limit the full-resolution decode would exhaust
// process memory with no useful thumbnail output.
var maxDecodePixels = int64(16 * 1024 * 1024) // 16 MP

// decodeImage reads a JPEG, PNG or WebP file. The image header is checked
// against maxDecodePixels before the full-resolution decode so oversized
// inputs fail predictably and cheaply; normal images are fully decoded.
func decodeImage(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	cfg, _, err := image.DecodeConfig(file)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	if pixels := int64(cfg.Width) * int64(cfg.Height); pixels > maxDecodePixels {
		return nil, fmt.Errorf("decode %s: %w (%dx%d > %d px)", path, ErrOversizedImage,
			cfg.Width, cfg.Height, maxDecodePixels)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	img, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return img, nil
}

// resizeImage scales an image to fit maxDimension without enlarging it.
func resizeImage(src image.Image, maxDimension int) image.Image {
	if maxDimension < 1 {
		return src
	}
	bounds := src.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 {
		return src
	}
	scale := math.Min(1, float64(maxDimension)/float64(max(width, height)))
	targetWidth := max(1, int(math.Round(float64(width)*scale)))
	targetHeight := max(1, int(math.Round(float64(height)*scale)))
	if targetWidth == width && targetHeight == height {
		return src
	}
	dst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)
	return dst
}
