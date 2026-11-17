package thumbnail

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

// runFFmpeg starts ffmpeg, registers it with the throttler and collects its
// output. Throttled processes stay suspended until Resume, including one
// started while the gate was already closed.
func runFFmpeg(ctx context.Context, ffmpeg string, throttle *Throttler, stdin io.Reader, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, ffmpeg, args...)
	cmd.Stdin = stdin
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", ffmpeg, err)
	}
	unregister := throttle.register(cmd)
	defer unregister()

	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("ffmpeg: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	if stdout.Len() == 0 {
		return nil, errors.New("ffmpeg produced no image")
	}
	return stdout.Bytes(), nil
}

// encodeWebP writes an RGBA image to ffmpeg's WebP encoder.
func encodeWebP(ctx context.Context, ffmpeg string, img image.Image, quality int, throttle *Throttler) ([]byte, error) {
	bounds := img.Bounds()
	rgba := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(rgba, rgba.Bounds(), img, bounds.Min, draw.Src)
	args := []string{
		"-f", "rawvideo", "-pix_fmt", "rgba",
		"-s", fmt.Sprintf("%dx%d", rgba.Bounds().Dx(), rgba.Bounds().Dy()),
		"-i", "pipe:0", "-frames:v", "1",
		"-c:v", "libwebp", "-quality", strconv.Itoa(quality),
		"-f", "webp", "pipe:1",
	}
	return runFFmpeg(ctx, ffmpeg, throttle, bytes.NewReader(rgba.Pix), args...)
}
