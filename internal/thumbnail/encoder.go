package thumbnail

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/png"
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

// webpEncoder owns one long-lived ffmpeg process. Reusing the process avoids
// paying ffmpeg's startup cost for every thumbnail. PNG frames carry each
// image because, unlike rawvideo, image2pipe accepts varying dimensions.
type webpEncoder struct {
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	reader   *bufio.Reader
	requests chan encodeRequest
	stop     chan struct{}
	done     chan struct{}
	once     sync.Once
}

func newWebPEncoder(ffmpeg string, quality int, throttle *Throttler) (*webpEncoder, error) {
	cmd := exec.Command(ffmpeg,
		"-loglevel", "error",
		"-f", "image2pipe", "-c:v", "png", "-i", "pipe:0",
		"-c:v", "libwebp", "-quality", strconv.Itoa(quality),
		"-f", "image2pipe", "pipe:1",
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
	var frame bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := encoder.Encode(&frame, img); err != nil {
		return nil, err
	}
	if _, err := e.stdin.Write(frame.Bytes()); err != nil {
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

// webpPool round-robins requests over a few encoders. If ffmpeg cannot be
// started, callers fall back to a one-shot encode.
type webpPool struct {
	encoders []*webpEncoder
	next     atomic.Uint64
}

func newWebPPool(count int, ffmpeg string, quality int, throttle *Throttler) *webpPool {
	pool := &webpPool{}
	for i := 0; i < count; i++ {
		encoder, err := newWebPEncoder(ffmpeg, quality, throttle)
		if err != nil {
			break
		}
		pool.encoders = append(pool.encoders, encoder)
	}
	return pool
}

func (p *webpPool) encode(ctx context.Context, img image.Image) ([]byte, error) {
	if len(p.encoders) == 0 {
		return nil, errors.New("no WebP encoders available")
	}
	index := p.next.Add(1) % uint64(len(p.encoders))
	request := encodeRequest{image: img, result: make(chan encodeResult, 1)}
	select {
	case p.encoders[index].requests <- request:
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
	for _, encoder := range p.encoders {
		encoder.close()
	}
}
