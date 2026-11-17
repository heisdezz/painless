# UI Feature 04: High-Res Photo Viewer & Native Video Player

## 1. Overview

The Media Viewer provides detailed inspection of individual media assets. It leverages Lobe UI components for full-fidelity photo zoom/pan inspection (`display.Image`), collapsible metadata drawers (`feedback.Drawer` / `feedback.FloatingPanel`), action toolbars (`action.ActionIconGroup`), and a native video playback interface.

---

## 2. Photo Viewer (`internal/ui/screens/viewer/image_viewer.go`)

- **Interactive Zoom & Pan**:
  - Leverages `github.com/lobehub/lobe-mygo/pkg/display.Image`:
    - Shows picture in a rounded frame that opens in an interactive preview viewer.
    - Zooms with mouse wheel, trackpad pinch, double-click, or toolbar buttons.
    - Pans smoothly when dragged.
- **Top / Bottom Floating Toolbar**:
  - `action.ActionIconGroup` containing:
    - Previous Item (`lucide.ChevronLeft`)
    - Next Item (`lucide.ChevronRight`)
    - Zoom In (`lucide.ZoomIn`)
    - Zoom Out (`lucide.ZoomOut`)
    - Reset Zoom (`lucide.RotateCcw`)
    - Toggle Metadata Drawer (`lucide.Info`)
    - Close Viewer (`lucide.X`)

---

## 3. Native Video Player (`internal/ui/screens/viewer/video_player.go`)

### Video Decoding Architecture

- Video playback uses `mygo/ui.VideoPlayer` driven by a custom `ui.VideoSeeker` implementation (`internal/ui/screens/viewer/ffmpeg_decoder.go`).
- The desktop viewer opens the cataloged file on disk directly; it does not use the mobile HTTP playback endpoint.
- The decoder spawns an `ffmpeg` frame extraction process:
  `ffmpeg -ss <offset> -i <path> -f rawvideo -pix_fmt rgba -s <w>x<h> -`
- Uncompressed frames are piped directly to `mygo/ui.VideoFrame`.
- **CPU Throttling Hook**: When playback begins, emit `VideoPlaybackStarted` event to pause background thumbnail generation; resume on stop/pause.

### Player Controls

- Play / Pause toggle (`action.ActionIcon` with `lucide.Play` / `lucide.Pause`).
- Scrubber slider: `github.com/lobehub/lobe-mygo/pkg/field.Slider` with thin track and pill thumb.
- Volume & Mute control (`lucide.Volume2` / `lucide.VolumeX`).
- Current time & total duration formatted display (`display.Tag`).

---

## 4. Metadata Inspector Side Drawer (`internal/ui/screens/viewer/metadata_panel.go`)

Uses `github.com/lobehub/lobe-mygo/pkg/feedback.Drawer` or `feedback.FloatingPanel`:

- Slides in from the right edge over a dimmed backdrop.
- Sections structured with `display.Accordion`:
  - **File Attributes**: Name, relative path, file size, MIME type, SHA-256 hash.
  - **Dimensions & Format**: Resolution, aspect ratio, duration, bitrate.
  - **EXIF Camera Details**: Camera Make & Model, Lens, Aperture ($f$-stop), Exposure Time, ISO, Focal Length.
  - **GPS Location**: Latitude, Longitude, Altitude (with copy button via `action.CopyButton`).

---

## 5. File Structure

- `internal/ui/screens/viewer/viewer_screen.go`: Main viewer container and routing.
- `internal/ui/screens/viewer/image_viewer.go`: Photo zoom/pan widget via `display.Image`.
- `internal/ui/screens/viewer/video_player.go`: Video player UI and playback controls.
- `internal/ui/screens/viewer/ffmpeg_decoder.go`: Ffmpeg pipe implementer of `ui.VideoSeeker`.
- `internal/ui/screens/viewer/metadata_panel.go`: `feedback.Drawer` displaying EXIF attributes.

---

## 6. Acceptance Criteria

- [ ] Photos open in interactive preview viewer with mouse wheel zoom and pan.
- [ ] Left/Right arrows navigate seamlessly between media items.
- [ ] Video playback triggers CPU thumbnail pause and responds to scrubber seeks.
- [ ] EXIF metadata slides out cleanly using `feedback.Drawer`.
