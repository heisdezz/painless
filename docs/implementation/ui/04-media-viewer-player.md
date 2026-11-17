# UI Feature 04: High-Res Photo Viewer & Native Video Player

## 1. Overview
The Media Viewer provides focused inspection of individual media assets. It includes an interactive photo viewer with smooth panning and zooming, an inspector side panel for EXIF and video metadata, and a native video player.

---

## 2. Photo Viewer (`internal/ui/screens/viewer/image_viewer.go`)

- **Interactive Zoom & Pan**:
  - Uses `ui.Zoom` or custom transform handlers.
  - Mouse wheel zooms in/out centered at mouse pointer.
  - Click-and-drag pans when zoomed in.
  - Double-click toggles between "Fit to Screen" and "100% Actual Pixels".
- **Previous / Next Navigation**:
  - On-screen left/right arrow overlays.
  - Keyboard shortcuts: `ArrowLeft` (previous item) and `ArrowRight` (next item).

---

## 3. Native Video Player (`internal/ui/screens/viewer/video_player.go`)

### The Video Decoder Challenge in Pure Go
In webviews, WebKit decodes H.264/HEVC/WebM. In MyGo native UI, `ui.VideoPlayer` requires a `ui.VideoDecoder` and `ui.VideoSeeker`:

```go
type VideoDecoder interface {
    Next() (ui.VideoFrame, error)
    Rewind() error
}

type VideoSeeker interface {
    VideoDecoder
    Seek(d time.Duration) (time.Duration, error)
}
```

### Video Implementation Strategy
1. **Ffmpeg Frame Pipeline (`internal/ui/screens/viewer/ffmpeg_decoder.go`)**:
   - Spawn an `ffmpeg` worker reading the requested video:
     `ffmpeg -ss <offset> -i <path> -f rawvideo -pix_fmt rgba -s <w>x<h> -`
   - Read uncompressed RGBA frames from stdout pipe directly into `image.RGBA` / `ui.VideoFrame`.
   - Implement `Seek(d)` by restarting the pipe with `-ss <new_time>`.
2. **Audio Sync**:
   - Audio decoded to PCM and sent to native audio output (e.g., via `github.com/ebitengine/oto/v3` or native pipe).
3. **Player Controls**:
   - Play / Pause button (`Spacebar` shortcut).
   - Time scrubber slider (`ui.Slider` or progress bar) showing current time / total duration.
   - Volume slider and mute toggle.

---

## 4. Metadata Inspector Side Panel (`internal/ui/screens/viewer/metadata_panel.go`)

Collapsible drawer displaying:
- **File Info**: File name, original path, file size (MB/GB), MIME type, SHA-256 hash.
- **Image / Video Info**: Dimensions (width x height, megapixels), aspect ratio, video bitrate, framerate.
- **EXIF Data**: Date taken, camera make & model, lens, exposure time, aperture (f-number), ISO, focal length.
- **Location**: GPS latitude, longitude, and elevation (with copyable coordinates or OpenStreetMap link).

---

## 5. File Structure
- `internal/ui/screens/viewer/viewer_screen.go`: Main viewer container and layout split.
- `internal/ui/screens/viewer/image_viewer.go`: Photo zoom/pan widget.
- `internal/ui/screens/viewer/video_player.go`: Video player UI and controls.
- `internal/ui/screens/viewer/ffmpeg_decoder.go`: Ffmpeg pipe implementer of `ui.VideoSeeker`.
- `internal/ui/screens/viewer/metadata_panel.go`: Collapsible metadata drawer.

---

## 6. Acceptance Criteria
- [ ] Photos load in full resolution with smooth zoom and pan controls.
- [ ] Left/Right arrow keys navigate seamlessly between media items.
- [ ] Video playback starts within 200ms and responds to Play/Pause and Seek.
- [ ] EXIF metadata renders neatly formatted values.
