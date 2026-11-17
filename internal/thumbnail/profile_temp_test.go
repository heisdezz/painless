package thumbnail

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"path/filepath"
	"testing"
)

func BenchmarkDecodeResizeOnly(b *testing.B) {
	root := b.TempDir()
	path := filepath.Join(root, "bench.jpg")
	writeTestJPEG(b, path, 1200, 800)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		img, err := decodeImage(path)
		if err != nil {
			b.Fatal(err)
		}
		_ = resizeImage(img, 320)
	}
}

func BenchmarkEncodeOnly(b *testing.B) {
	img := image.NewRGBA(image.Rect(0, 0, 320, 213))
	width, height := 320, 213
	_ = width
	_ = height
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, img, nil); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := encodeWebP(context.Background(), "ffmpeg", img, 80, NewThrottler()); err != nil {
			b.Fatal(err)
		}
	}
}
