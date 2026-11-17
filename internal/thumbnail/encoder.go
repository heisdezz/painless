package thumbnail

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"io"
	"os/exec"
	"strconv"
	"sync"
	"sync/atomic"
)

// encodeRequest is one image sent to a persistent WebP encoder.
type encodeRequest struct {
	image  image.Image
	result chan encodeResult
}

type encodeResult struct {
	data []byte
	err  error
}

// webpEncoder owns one long-lived ffmpeg process for a fixed frame size.
// Rawvideo streams a frame as soon as it is written, so the process can be
// reused instead of paying ffmpeg's startup cost for every thumbnail.
type webpEncoder struct {
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	reader   *bufio.Reader
	requests chan encodeRequest
	stop     chan struct{}
	done     chan struct{}
	busy     atomic.Int32
	once     sync.Once
}

func newWebPEncoder(ffmpeg string, width, height, quality int, throttle *Throttler) (*webpEncoder, error) {
	cmd := exec.Command(ffmpeg,
		"-loglevel", "error",
		"-f", "rawvideo", "-pix_fmt", "rgba",
		"-s", fmt.Sprintf("%dx%d", width, height), "-i", "pipe:0",
		"-c:v", "libwebp", "-quality", strconv.Itoa(quality),
		"-f", "image2pipe", "-flush_packets", "1", "pipe:1",
	)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, err
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		stdin.Close()
		return nil, err
	}
	unregister := throttle.register(cmd)
	encoder := &webpEncoder{
		cmd:      cmd,
		stdin:    stdin,
		reader:   bufio.NewReader(stdout),
		requests: make(chan encodeRequest),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	go func() {
		defer close(encoder.done)
		defer stdin.Close()
		defer unregister()
		_ = cmd.Wait()
	}()
	go encoder.run()
	return encoder, nil
}

func (e *webpEncoder) run() {
	var failed error
	for {
		select {
		case <-e.stop:
			return
		case request := <-e.requests:
			if failed != nil {
				request.result <- encodeResult{err: failed}
				continue
			}
			var data []byte
			data, failed = e.encode(request.image)
			request.result <- encodeResult{data: data, err: failed}
		}
	}
}

func (e *webpEncoder) encode(img image.Image) ([]byte, error) {
	bounds := img.Bounds()
	rgba := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(rgba, rgba.Bounds(), img, bounds.Min, draw.Src)
	if _, err := e.stdin.Write(rgba.Pix); err != nil {
		return nil, err
	}
	header := make([]byte, 12)
	if _, err := io.ReadFull(e.reader, header); err != nil {
		return nil, err
	}
	if string(header[:4]) != "RIFF" || string(header[8:12]) != "WEBP" {
		return nil, errors.New("unexpected WebP frame from ffmpeg")
	}
	size := int(binary.LittleEndian.Uint32(header[4:8])) + 8
	if size < 12 || size > 64<<20 {
		return nil, errors.New("invalid WebP frame size from ffmpeg")
	}
	data := make([]byte, size)
	copy(data, header)
	if _, err := io.ReadFull(e.reader, data[12:]); err != nil {
		return nil, err
	}
	return data, nil
}

func (e *webpEncoder) close() {
	e.once.Do(func() {
		close(e.stop)
		if e.cmd.Process != nil {
			_ = e.cmd.Process.Kill()
		}
	})
}

// webpPool keeps a small number of encoders keyed by frame size. Requests
// for an uncached size start one; idle encoders are evicted least-recently.
type webpPool struct {
	mu       sync.Mutex
	encoders map[string]*webpEncoder
	recent   []string
	max      int
	ffmpeg   string
	quality  int
	throttle *Throttler
}

func newWebPPool(max int, ffmpeg string, quality int, throttle *Throttler) *webpPool {
	if max < 1 {
		max = 1
	}
	return &webpPool{
		encoders: make(map[string]*webpEncoder, max),
		max:      max,
		ffmpeg:   ffmpeg,
		quality:  quality,
		throttle: throttle,
	}
}

func (p *webpPool) encoderFor(width, height int) (*webpEncoder, error) {
	key := fmt.Sprintf("%dx%d", width, height)
	p.mu.Lock()
	defer p.mu.Unlock()
	if encoder := p.encoders[key]; encoder != nil {
		p.touch(key)
		return encoder, nil
	}
	encoder, err := newWebPEncoder(p.ffmpeg, width, height, p.quality, p.throttle)
	if err != nil {
		return nil, err
	}
	p.encoders[key] = encoder
	p.touch(key)
	for len(p.encoders) > p.max {
		victim := p.evictable()
		if victim == "" {
			break
		}
		p.encoders[victim].close()
		delete(p.encoders, victim)
		p.recent = removeKey(p.recent, victim)
	}
	return encoder, nil
}

func (p *webpPool) encode(ctx context.Context, img image.Image) ([]byte, error) {
	bounds := img.Bounds()
	encoder, err := p.encoderFor(bounds.Dx(), bounds.Dy())
	if err != nil {
		return nil, err
	}
	encoder.busy.Add(1)
	defer encoder.busy.Add(-1)
	request := encodeRequest{image: img, result: make(chan encodeResult, 1)}
	select {
	case encoder.requests <- request:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	select {
	case result := <-request.result:
		return result.data, result.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (p *webpPool) close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for key, encoder := range p.encoders {
		encoder.close()
		delete(p.encoders, key)
	}
	p.recent = nil
}

func (p *webpPool) touch(key string) {
	p.recent = removeKey(p.recent, key)
	p.recent = append(p.recent, key)
}

func (p *webpPool) evictable() string {
	for i := len(p.recent) - 1; i >= 0; i-- {
		key := p.recent[i]
		if p.encoders[key].busy.Load() == 0 {
			return key
		}
	}
	return ""
}

func removeKey(keys []string, key string) []string {
	for i, existing := range keys {
		if existing == key {
			return append(keys[:i], keys[i+1:]...)
		}
	}
	return keys
}
