package viewer

import (
	"fmt"
	"math"
	"path/filepath"
	"time"

	mui "github.com/egoist/mygo/ui"
	uivideo "github.com/egoist/mygo/ui/video"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/choice"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/domain"
)

const (
	// maxVideoZoom bounds zooming so a few notches cannot blow the frame up
	// into an unbounded texture, and zooming back out always reaches 1.
	maxVideoZoom = 8
	// videoZoomStep is one wheel notch: a quarter of the current size.
	videoZoomStep = 1.25
)

func (s *Screen) renderVideoPlayer(c *mui.Context, tk *lobetheme.Tokens, item *domain.MediaItem) {
	c.Key(item.FileHash)

	s.ensureVideoInitialized(item)

	// Theater stage: the frame fills the dark canvas and the slim control
	// strip sits at its foot, without a floating card around it.
	mui.Column(c).Grow(1).Basis(0).Children(func() {
		// Clip keeps a zoomed frame inside the stage instead of letting it
		// paint over the controls or the rest of the window.
		s.mu.RLock()
		zoomed := s.videoZoom > 1
		panning := s.videoPanning
		s.mu.RUnlock()
		cursor := mui.CursorDefault
		if zoomed {
			cursor = mui.CursorGrab
		}
		if panning {
			cursor = mui.CursorGrabbing
		}
		stageBox := mui.Box(c).Grow(1).Basis(0).Center().Clip().Cursor(cursor)
		// Take the wheel and drag anywhere on the stage, including the
		// letterboxed margins: the video element itself may not be under the
		// pointer, and the pressed element keeps the drag moves.
		stageBox.HandleInput(s.handleVideoInput).Children(func() {
			s.mu.RLock()
			loading, videoErr := s.videoLoading, s.videoErr
			zoom, panX, panY := s.videoZoom, s.videoPanX, s.videoPanY
			s.mu.RUnlock()
			if s.videoPlayer != nil {
				// Contain keeps the player's aspect ratio while filling the
				// stage; the system engine decodes at the source ratio.
				// Scaling about the box's top-left and panning by the
				// pointer-anchored offset zooms into the pixel under the
				// mouse, not the middle of the frame.
				mui.Video(c, s.videoPlayer).Fill().Fit(mui.Contain).
					Transform(mui.Scaling(zoom, zoom).Then(mui.Translation(panX, panY))).
					TransformOrigin(0, 0)
				// Keep the spinner up until the first picture is ready. The
				// Video element above is still built, so the engine can
				// load and wake the frame that takes it away.
				if loading || !s.videoPlayer.Ready() {
					renderVideoLoading(c, tk, "Loading video…")
				}
			} else if loading {
				renderVideoLoading(c, tk, "Loading video…")
			} else if videoErr != "" {
				mui.Text(c, videoErr).FontSize(13).TextColor(tk.TextTertiary)
			} else {
				mui.Text(c, "No video preview available").
					FontSize(13).TextColor(tk.TextTertiary)
			}
		})
		s.videoControlsBar(c, tk, item)
	})
}

// renderVideoLoading overlays a quiet spinner while a player opens or waits
// for its first frame, so the stage doesn't claim there is no video yet.
func renderVideoLoading(c *mui.Context, tk *lobetheme.Tokens, label string) {
	mui.Column(c).Absolute().Fill().Center().Gap(12).PassThrough().Children(func() {
		spin := mui.Icon(c, lucide.LoaderCircle()).FontSize(28).TextColor(tk.TextTertiary)
		spin.Rotate(spin.Loop("video-loading", time.Second, mui.Linear) * 360)
		mui.Text(c, label).FontSize(12).TextColor(tk.TextTertiary)
	})
}

// handleVideoInput zooms on wheel notches, two-finger scroll and pinches,
// all at the pointer. X and Y here are element-local, as MyGo reports them.
func (s *Screen) handleVideoInput(ev mui.InputEvent) bool {
	switch ev.Kind {
	case mui.InputPointerDown:
		if ev.Button == 0 {
			// Take the press so MyGo keeps sending its moves and release
			// to the stage even after the pointer leaves it. Panning is a
			// plain drag, but only while the frame is zoomed in.
			s.mu.Lock()
			s.videoPointerDown = true
			s.videoPanning = s.videoZoom > 1
			s.videoDragX, s.videoDragY = ev.X, ev.Y
			s.videoPanStartX, s.videoPanStartY = s.videoPanX, s.videoPanY
			s.mu.Unlock()
			return true
		}
	case mui.InputPointerMove:
		s.mu.RLock()
		down := s.videoPointerDown
		s.mu.RUnlock()
		if down && ev.Button == 0 {
			s.mu.Lock()
			if s.videoZoom > 1 {
				// Pointer moves arrive in element-local (pre-transform)
				// coords; pan is a screen-space offset. Convert the drag
				// by the zoom so the picture follows the cursor exactly,
				// and the point under it is what the next zoom anchors on.
				zoom := s.videoZoom
				s.videoPanning = true
				s.videoPanX += (ev.X - s.videoDragX) * zoom
				s.videoPanY += (ev.Y - s.videoDragY) * zoom
			} else {
				// Back at 1x there is nothing to pan over.
				s.videoPanning = false
			}
			s.videoDragX, s.videoDragY = ev.X, ev.Y
			s.mu.Unlock()
			s.requestRedraw()
			return true
		}
	case mui.InputPointerUp:
		s.mu.RLock()
		down := s.videoPointerDown
		s.mu.RUnlock()
		if down && ev.Button == 0 {
			s.mu.Lock()
			s.videoPointerDown = false
			s.videoPanning = false
			if ev.Canceled {
				s.videoPanX, s.videoPanY = s.videoPanStartX, s.videoPanStartY
			}
			s.mu.Unlock()
			s.requestRedraw()
			return true
		}
	case mui.InputZoom:
		if ev.Scale > 0 {
			s.zoomVideoAt(ev.X, ev.Y, ev.Scale)
			return true
		}
	case mui.InputScroll:
		if ev.DY == 0 {
			return false
		}
		factor := float32(1)
		if ev.Precise {
			// A touchpad sends many small, continuous deltas; map them to
			// a smooth zoom instead of one notch per event.
			factor = float32(math.Exp(-float64(ev.DY) / 240))
		} else if ev.DY < 0 {
			factor = videoZoomStep
		} else {
			factor = 1 / videoZoomStep
		}
		s.zoomVideoAt(ev.X, ev.Y, factor)
		return true
	}
	return false
}

// zoomVideoAt scales the frame by factor. Zooming in keeps the picture point
// under the pointer fixed (MyGo reports it in element-local coords, so its
// screen offset is pan + zoom*p). Zooming out keeps the *original* zoom-in
// anchor instead of the cursor: pan grows as zoom-1, so scaling it by the
// change in zoom-1 retraces the zoom. The frame returns to center at 1x no
// matter where the cursor is when the user zooms out.
func (s *Screen) zoomVideoAt(x, y, factor float32) {
	if factor <= 0 || factor == 1 {
		return
	}
	s.mu.Lock()
	oldZoom := s.videoZoom
	zoom := oldZoom * factor
	if zoom < 1 {
		zoom, factor = 1, 1/oldZoom
	} else if zoom > maxVideoZoom {
		zoom, factor = maxVideoZoom, maxVideoZoom/oldZoom
	}
	if factor != 1 {
		if factor < 1 {
			// Retrace the existing zoom rather than re-anchoring at the
			// cursor, which would displace the frame on the way out.
			scale := float32(0)
			if oldZoom > 1 {
				scale = (zoom - 1) / (oldZoom - 1)
			}
			s.videoPanX *= scale
			s.videoPanY *= scale
		} else {
			// The cursor's screen offset is pan + zoom*p. Keep that point
			// at the same offset after the zoom, wherever we panned to.
			anchorX := s.videoPanX + oldZoom*x
			anchorY := s.videoPanY + oldZoom*y
			s.videoPanX, s.videoPanY = anchorX-zoom*x, anchorY-zoom*y
		}
		s.videoZoom = zoom
	}
	s.mu.Unlock()
	s.requestRedraw()
}

// resetVideoZoom returns the frame to its natural size.
func (s *Screen) resetVideoZoom() {
	s.mu.Lock()
	s.videoZoom, s.videoPanX, s.videoPanY = 1, 0, 0
	s.videoPanning = false
	s.videoPointerDown = false
	s.mu.Unlock()
	s.requestRedraw()
}

func (s *Screen) ensureVideoInitialized(item *domain.MediaItem) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.videoHash == item.FileHash {
		return
	}

	s.stopVideoLocked()
	s.videoHash = item.FileHash
	s.videoLoading = true
	s.videoErr = ""

	var fullPath string
	if s.driveRoot != "" {
		fullPath = filepath.Join(s.driveRoot, item.CurrentRelativePath)
	} else {
		fullPath = item.CurrentRelativePath
	}

	opener := s.videoOpener
	if opener == nil {
		opener = uivideo.Open
	}
	player, err := opener(fullPath)
	if err != nil {
		s.videoErr = "No video preview available"
		s.videoLoading = false
		return
	}
	// The player starts playing as soon as it is shown; with autoplay off,
	// open it paused so the first frame is there but nothing runs until the
	// user presses Play. stopVideoLocked already resumed the throttler.
	if s.autoplay != nil && !s.autoplay() {
		player.Pause()
	}
	s.videoPlayer = player
	s.videoLoading = false
}

func (s *Screen) videoControlsBar(c *mui.Context, tk *lobetheme.Tokens, item *domain.MediaItem) {
	mui.Row(c).Shrink(0).Padding(10, 16).AlignItems(mui.Center).Gap(12).Children(func() {
		// Play/Pause button
		isPlaying := false
		if s.videoPlayer != nil {
			isPlaying = s.videoPlayer.Playing()
		}

		playIcon := lucide.Play()
		playTitle := "Play"
		if isPlaying {
			playIcon = lucide.Pause()
			playTitle = "Pause"
		}

		action.ActionIcon(c, playIcon, action.ActionIconProps{
			Size:  lobetheme.Middle,
			Title: playTitle,
		}).TextColor(tk.TextTertiary).OnClick(func() {
			s.togglePlayPause()
		})

		// Current Position and Duration
		var curPos, totalDur float64
		if s.videoPlayer != nil {
			curPos = s.videoPlayer.Position().Seconds()
			totalDur = s.videoPlayer.Duration().Seconds()
		}
		if totalDur <= 0 && item.DurationSeconds != nil {
			totalDur = float64(*item.DurationSeconds)
		}

		mui.Text(c, formatSeconds(curPos)+" / "+formatSeconds(totalDur)).
			FontSize(12).TextColor(tk.TextTertiary).Shrink(0).SingleLine()

		// Scrubber slider. While dragged, the slider keeps its own value; the
		// player seeks as the value settles instead of on every pixel.
		sliderPos := curPos
		if !s.seekAt.IsZero() && time.Since(s.seekAt) < time.Second {
			sliderPos = s.seekPos
		}
		slider := choice.Slider(c, &sliderPos, choice.SliderProps{
			Min: 0,
			Max: totalDur,
		}).Grow(1)
		pressed := slider.Pressed()
		if pressed {
			s.scrubbing = true
		}
		if slider.Changed() {
			s.seekPos, s.seekAt = sliderPos, time.Now()
			if !s.scrubbing && s.videoPlayer != nil {
				s.videoPlayer.Seek(time.Duration(sliderPos * float64(time.Second)))
			}
		}
		if s.scrubbing && !pressed {
			s.scrubbing = false
			s.seekPos, s.seekAt = sliderPos, time.Now()
			if s.videoPlayer != nil {
				s.videoPlayer.Seek(time.Duration(sliderPos * float64(time.Second)))
			}
		}

		// Volume/Mute button
		isMuted := false
		if s.videoPlayer != nil {
			isMuted = s.videoPlayer.Muted()
		}

		muteIcon := lucide.Volume2()
		muteTitle := "Mute"
		if isMuted {
			muteIcon = lucide.VolumeX()
			muteTitle = "Unmute"
		}

		action.ActionIcon(c, muteIcon, action.ActionIconProps{
			Size:  lobetheme.Middle,
			Title: muteTitle,
		}).TextColor(tk.TextTertiary).OnClick(func() {
			if s.videoPlayer != nil {
				s.videoPlayer.SetMuted(!isMuted)
				s.requestRedraw()
			}
		})

		// Zoom readout doubles as a reset: the current scale is visible, and
		// a click returns to the full frame without a reverse wheel gesture.
		s.mu.RLock()
		zoom := s.videoZoom
		s.mu.RUnlock()
		action.Button(c, fmt.Sprintf("%d%%", int(zoom*100)), action.ButtonProps{
			Variant: action.Text, Size: lobetheme.Small,
		}).Shrink(0).OnClick(func() { s.resetVideoZoom() })
	})
}

func (s *Screen) togglePlayPause() {
	if s.videoPlayer == nil {
		// If video player isn't active (e.g. test or unsupported), still toggle throttler for testing
		s.mu.Lock()
		if s.throttler != nil {
			s.throttler.Pause()
		}
		s.mu.Unlock()
		s.requestRedraw()
		return
	}

	if s.videoPlayer.Playing() {
		s.videoPlayer.Pause()
		if s.throttler != nil {
			s.throttler.Resume()
		}
	} else {
		s.videoPlayer.Play()
		if s.throttler != nil {
			s.throttler.Pause()
		}
	}
	s.requestRedraw()
}

func (s *Screen) stopVideo() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopVideoLocked()
}

func (s *Screen) stopVideoLocked() {
	if s.videoPlayer != nil {
		s.videoPlayer.Pause()
		s.videoPlayer.Close()
		s.videoPlayer = nil
	}
	s.videoHash = ""
	// A new frame starts at its natural size; zoom belongs to the video it
	// was applied to, not to the screen or the next item.
	s.videoZoom, s.videoPanX, s.videoPanY = 1, 0, 0
	s.videoPanning = false
	s.videoPointerDown = false
	s.videoLoading = false
	s.videoErr = ""

	if s.throttler != nil {
		s.throttler.Resume()
	}
}

func formatSeconds(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	s := int(sec)
	m := s / 60
	s = s % 60
	return fmt.Sprintf("%02d:%02d", m, s)
}
