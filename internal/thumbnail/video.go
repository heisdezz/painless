package thumbnail

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

// extractVideoFrame grabs an early representative frame and encodes it
// directly to WebP. When the seek point is past the end of a short clip, it
// retries from the first frame.
func extractVideoFrame(ctx context.Context, ffmpeg, source string, seek time.Duration, maxDimension, quality int, throttle *Throttler) ([]byte, error) {
	filter := fmt.Sprintf("scale=w='min(%d,iw)':h=-2", maxDimension)
	args := []string{"-y"}
	seekAt := func(at time.Duration) []string {
		return []string{
			"-ss", strconv.FormatFloat(at.Seconds(), 'f', -1, 64),
			"-i", source, "-frames:v", "1", "-vf", filter,
			"-c:v", "libwebp", "-quality", strconv.Itoa(quality),
			"-f", "webp", "pipe:1",
		}
	}
	data, err := runFFmpeg(ctx, ffmpeg, throttle, nil, append(args, seekAt(seek)...)...)
	if err == nil || seek <= 0 || ctx.Err() != nil {
		return data, err
	}
	return runFFmpeg(ctx, ffmpeg, throttle, nil, append(args, seekAt(0)...)...)
}
