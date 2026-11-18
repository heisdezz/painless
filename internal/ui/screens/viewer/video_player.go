package viewer

import (
	"fmt"
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

func (s *Screen) renderVideoPlayer(c *mui.Context, tk *lobetheme.Tokens, item *domain.MediaItem) {
	c.Key(item.FileHash)

	s.ensureVideoInitialized(item)

	// Theater stage: the frame fills the dark canvas and the slim control
	// strip sits at its foot, without a floating card around it.
	mui.Column(c).Grow(1).Basis(0).Children(func() {
		mui.Box(c).Grow(1).Basis(0).Center().Children(func() {
			if s.videoPlayer != nil {
				// Contain keeps the player's aspect ratio while filling the
				// stage; the system engine decodes at the source ratio.
				mui.Video(c, s.videoPlayer).Fill().Fit(mui.Contain)
			} else {
				mui.Text(c, "No video preview available").
					FontSize(13).TextColor(tk.TextTertiary)
			}
		})
		s.videoControlsBar(c, tk, item)
	})
}

func (s *Screen) ensureVideoInitialized(item *domain.MediaItem) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.videoHash == item.FileHash {
		return
	}

	s.stopVideoLocked()
	s.videoHash = item.FileHash

	var fullPath string
	if s.driveRoot != "" {
		fullPath = filepath.Join(s.driveRoot, item.CurrentRelativePath)
	} else {
		fullPath = item.CurrentRelativePath
	}

	player, err := uivideo.Open(fullPath)
	if err != nil {
		return
	}
	s.videoPlayer = player
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
