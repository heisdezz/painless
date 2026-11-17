package thumbnail

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"golang.org/x/image/webp"

	"painless/internal/domain"
)

func writeTestJPEG(t testing.TB, path string, width, height int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 128, A: 255})
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := jpeg.Encode(file, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
}

func assertWebP(t *testing.T, data []byte, maxDimension int) {
	t.Helper()
	if len(data) < 16 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		t.Fatalf("not a WebP image: % q", data[:min(len(data), 16)])
	}
	config, err := webp.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode WebP: %v", err)
	}
	if config.Width > maxDimension || config.Height > maxDimension {
		t.Fatalf("thumbnail too large: %dx%d", config.Width, config.Height)
	}
}

func testItem(path, hash, mime string) domain.MediaItem {
	return domain.MediaItem{
		FileHash:            hash,
		CurrentRelativePath: path,
		MimeType:            mime,
	}
}

func TestImageThumbnailAndDiskCache(t *testing.T) {
	root := t.TempDir()
	writeTestJPEG(t, filepath.Join(root, "photos", "a.jpg"), 640, 480)
	service := New(Options{DriveRoot: root, MaxDimension: 320})
	defer service.Close()
	item := testItem(filepath.Join("photos", "a.jpg"), "abcdef123456", "image/jpeg")

	data, err := service.Get(context.Background(), item)
	if err != nil {
		t.Fatal(err)
	}
	assertWebP(t, data, 320)

	path := filepath.Join(root, ".media_thumbnails", "ab", "abcdef123456.webp")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("thumbnail not cached on disk at %s: %v", path, err)
	}

	// Removing the source proves the second call is served from the cache.
	if err := os.Remove(filepath.Join(root, "photos", "a.jpg")); err != nil {
		t.Fatal(err)
	}
	again, err := service.Get(context.Background(), item)
	if err != nil || !bytes.Equal(data, again) {
		t.Fatalf("cached read: %d bytes, err %v", len(again), err)
	}

	// A fresh service must find the same thumbnail on disk.
	reopened := New(Options{DriveRoot: root, MaxDimension: 320})
	defer reopened.Close()
	fromDisk, err := reopened.Get(context.Background(), item)
	if err != nil || !bytes.Equal(data, fromDisk) {
		t.Fatalf("disk cache read: %d bytes, err %v", len(fromDisk), err)
	}

	reopened.Invalidate(item.FileHash)
	if _, err := reopened.Get(context.Background(), item); err == nil {
		t.Fatal("expected a miss after Invalidate")
	}
}

func TestGetPath(t *testing.T) {
	root := t.TempDir()
	writeTestJPEG(t, filepath.Join(root, "b.png"), 8, 8)
	item := testItem("b.png", "hash01", "image/jpeg")
	service := New(Options{DriveRoot: root})
	defer service.Close()
	path, err := service.GetPath(context.Background(), item)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, filepath.Join("ha", "hash01.webp")) {
		t.Fatalf("cache path = %q", path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestVideoThumbnail(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	root := t.TempDir()
	video := filepath.Join(root, "clip.mp4")
	command := exec.Command("ffmpeg", "-y", "-f", "lavfi",
		"-i", "testsrc=duration=2:size=160x120:rate=5",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-loglevel", "error", video)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("create test video: %v: %s", err, output)
	}

	service := New(Options{DriveRoot: root, MaxDimension: 320})
	defer service.Close()
	item := testItem("clip.mp4", "videohash", "video/mp4")
	data, err := service.Get(context.Background(), item)
	if err != nil {
		t.Fatal(err)
	}
	assertWebP(t, data, 320)
}

func TestThrottlerGate(t *testing.T) {
	root := t.TempDir()
	writeTestJPEG(t, filepath.Join(root, "c.jpg"), 100, 100)
	service := New(Options{DriveRoot: root})
	defer service.Close()
	item := testItem("c.jpg", "gatehash", "image/jpeg")

	service.Throttler().Pause()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()
	if _, err := service.Get(ctx, item); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("paused Get error = %v, want deadline exceeded", err)
	}

	service.Throttler().Resume()
	if _, err := service.Get(context.Background(), item); err != nil {
		t.Fatalf("Get after Resume: %v", err)
	}
}

func TestThrottlerSuspendsProcesses(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep not available")
	}
	throttle := NewThrottler()
	command := exec.Command("sleep", "10")
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	unregister := throttle.register(command)
	defer func() {
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
		unregister()
	}()

	pid := command.Process.Pid
	throttle.Pause()
	if err := waitForState(pid, "T"); err != nil {
		t.Fatalf("process not suspended: %v (state %q)", err, processState(pid))
	}
	throttle.Resume()
	if err := waitForState(pid, ""); err != nil {
		t.Fatalf("process still suspended: %v (state %q)", err, processState(pid))
	}
}

func processState(pid int) string {
	if runtime.GOOS != "linux" {
		return ""
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return ""
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return ""
	}
	return fields[2]
}

func waitForState(pid int, want string) error {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		state := processState(pid)
		if want == "" && state != "T" && state != "" {
			return nil
		}
		if state == want {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return errors.New("timed out")
}

func TestInvalidInputs(t *testing.T) {
	root := t.TempDir()
	service := New(Options{DriveRoot: root})
	defer service.Close()

	if _, err := service.Get(context.Background(), testItem("x.jpg", "", "image/jpeg")); !errors.Is(err, ErrNoHash) {
		t.Fatalf("empty hash error = %v", err)
	}
	if _, err := service.Get(context.Background(), testItem("../../etc/passwd", "h", "image/jpeg")); err == nil {
		t.Fatal("expected path traversal to be rejected")
	}
	if _, err := service.Get(context.Background(), testItem("/etc/passwd", "h", "image/jpeg")); err == nil {
		t.Fatal("expected absolute path to be rejected")
	}
	if _, err := service.Get(context.Background(), testItem("x.txt", "h", "text/plain")); !errors.Is(err, ErrUnsupportedType) {
		t.Fatalf("unsupported type error = %v", err)
	}
}

func BenchmarkImageThumbnail(b *testing.B) {
	root := b.TempDir()
	writeTestJPEG(b, filepath.Join(root, "bench.jpg"), 1200, 800)
	service := New(Options{DriveRoot: root, MaxDimension: 320})
	defer service.Close()
	item := testItem("bench.jpg", "benchhash", "image/jpeg")
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.Invalidate(item.FileHash)
		if _, err := service.Get(ctx, item); err != nil {
			b.Fatal(err)
		}
	}
}

func TestMemoryCacheEviction(t *testing.T) {
	cache := newMemoryCache(2)
	cache.put("a", []byte("1"))
	cache.put("b", []byte("2"))
	cache.get("a") // make a most recent
	cache.put("c", []byte("3"))
	if _, ok := cache.get("b"); ok {
		t.Fatal("b should have been evicted")
	}
	if _, ok := cache.get("a"); !ok {
		t.Fatal("a should still be cached")
	}
	if cache.len() != 2 {
		t.Fatalf("cache length = %d", cache.len())
	}
}
