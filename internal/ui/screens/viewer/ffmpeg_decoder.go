// Package viewer provides full-screen high-res photo inspection and native video playback.
package viewer

import (
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	mui "github.com/egoist/mygo/ui"
)

// FFmpegDecoderConfig configures the ffmpeg video frame extractor.
type FFmpegDecoderConfig struct {
	FilePath string
	Width    int
	Height   int
	FPS      float64
	Duration time.Duration
}

// FFmpegDecoder implements ui.VideoSeeker by piping uncompressed RGBA frames
// from an ffmpeg child process.
type FFmpegDecoder struct {
	cfg      FFmpegDecoderConfig
	mu       sync.Mutex
	cmd      *exec.Cmd
	pipe     io.ReadCloser
	cancel   context.CancelFunc
	frameLen int
	position time.Duration
	closed   bool
}

// NewFFmpegDecoder creates a new decoder for the given video file.
func NewFFmpegDecoder(cfg FFmpegDecoderConfig) (*FFmpegDecoder, error) {
	if cfg.FilePath == "" {
		return nil, errors.New("empty video file path")
	}
	if cfg.Width <= 0 {
		cfg.Width = 640
	}
	if cfg.Height <= 0 {
		cfg.Height = 360
	}
	if cfg.FPS <= 0 {
		cfg.FPS = 30
	}

	d := &FFmpegDecoder{
		cfg:      cfg,
		frameLen: cfg.Width * cfg.Height * 4,
	}

	if err := d.startProcess(0); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *FFmpegDecoder) startProcess(offset time.Duration) error {
	d.stopProcess()

	// Check if file exists to avoid hanging ffmpeg on nonexistent files
	if _, err := os.Stat(d.cfg.FilePath); err != nil {
		return fmt.Errorf("video file does not exist: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	offsetSec := offset.Seconds()

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-nostdin",
		"-ss", fmt.Sprintf("%.3f", offsetSec),
		"-i", d.cfg.FilePath,
		"-f", "rawvideo",
		"-pix_fmt", "rgba",
		"-s", fmt.Sprintf("%dx%d", d.cfg.Width, d.cfg.Height),
		"-r", fmt.Sprintf("%.2f", d.cfg.FPS),
		"-",
	)

	pipe, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("ffmpeg stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("start ffmpeg: %w", err)
	}

	d.cmd = cmd
	d.pipe = pipe
	d.cancel = cancel
	d.position = offset
	return nil
}

func (d *FFmpegDecoder) stopProcess() {
	if d.cancel != nil {
		d.cancel()
		d.cancel = nil
	}
	if d.pipe != nil {
		_ = d.pipe.Close()
		d.pipe = nil
	}
	if d.cmd != nil && d.cmd.Process != nil {
		_ = d.cmd.Process.Kill()
		_ = d.cmd.Wait()
		d.cmd = nil
	}
}

// Next reads and returns the next decoded frame.
func (d *FFmpegDecoder) Next() (mui.VideoFrame, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.closed {
		return mui.VideoFrame{}, io.EOF
	}

	if d.pipe == nil {
		return mui.VideoFrame{}, io.EOF
	}

	buf := make([]byte, d.frameLen)
	_, err := io.ReadFull(d.pipe, buf)
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return mui.VideoFrame{}, io.EOF
		}
		return mui.VideoFrame{}, err
	}

	img := &image.RGBA{
		Pix:    buf,
		Stride: d.cfg.Width * 4,
		Rect:   image.Rect(0, 0, d.cfg.Width, d.cfg.Height),
	}

	frameDuration := time.Duration(float64(time.Second) / d.cfg.FPS)
	d.position += frameDuration

	return mui.VideoFrame{
		Image:    img,
		Duration: frameDuration,
	}, nil
}

// Rewind seeks back to the beginning of the video.
func (d *FFmpegDecoder) Rewind() error {
	_, err := d.Seek(0)
	return err
}

// Seek seeks to the requested timestamp.
func (d *FFmpegDecoder) Seek(offset time.Duration) (time.Duration, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.closed {
		return 0, io.EOF
	}

	if offset < 0 {
		offset = 0
	}
	if d.cfg.Duration > 0 && offset > d.cfg.Duration {
		offset = d.cfg.Duration
	}

	if err := d.startProcess(offset); err != nil {
		return 0, err
	}

	return offset, nil
}

// Duration returns the total video duration.
func (d *FFmpegDecoder) Duration() time.Duration {
	return d.cfg.Duration
}

// Position returns the current playback offset.
func (d *FFmpegDecoder) Position() time.Duration {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.position
}

// Close terminates ffmpeg and releases resources.
func (d *FFmpegDecoder) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.closed = true
	d.stopProcess()
	return nil
}
