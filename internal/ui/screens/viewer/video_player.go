package viewer

import (
	"fmt"
	"path/filepath"
	"time"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/choice"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/domain"
)

func (s *Screen) renderVideoPlayer(c *mui.Context, tk *lobetheme.Tokens, item *domain.MediaItem) {
	c.Key(item.FileHash)

	s.ensureVideoInitialized(item)

	mui.Column(c).Grow(1).Basis(0).Center().Gap(12).Children(func() {
		// Video viewport
		if s.videoPlayer != nil {
			mui.Video(c, s.videoPlayer)
		} else {
			mui.Column(c).Grow(1).Basis(0).Center().Children(func() {
				mui.Text(c, "Video Player").FontSize(14).TextColor(tk.TextSecondary)
			})
		}

		// Controls bar
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

	var dur time.Duration
	if item.DurationSeconds != nil {
		dur = time.Duration(*item.DurationSeconds) * time.Second
	}

	cfg := FFmpegDecoderConfig{
		FilePath: fullPath,
		Width:    640,
		Height:   360,
		FPS:      30,
		Duration: dur,
	}

	decoder, err := NewFFmpegDecoder(cfg)
	if err == nil {
		s.videoDecoder = decoder
		player, err := mui.NewVideoPlayer(decoder)
		if err == nil {
			s.videoPlayer = player
		}
	}
}

func (s *Screen) videoControlsBar(c *mui.Context, tk *lobetheme.Tokens, item *domain.MediaItem) {
	mui.Row(c).Shrink(0).Padding(10, 16).Radius(10).Background(tk.BgElevated).
		Border(1, tk.BorderSecondary).AlignItems(mui.Center).Gap(12).Children(func() {
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

		if action.ActionIcon(c, playIcon, action.ActionIconProps{
			Size:  lobetheme.Middle,
			Title: playTitle,
		}).Clicked() {
			s.togglePlayPause()
		}

		// Current Position and Duration
		var curPos, totalDur float64
		if s.videoPlayer != nil {
			curPos = s.videoPlayer.Position().Seconds()
			totalDur = s.videoPlayer.Duration().Seconds()
		}
		if totalDur <= 0 && item.DurationSeconds != nil {
			totalDur = float64(*item.DurationSeconds)
		}

		curFormatted := formatSeconds(curPos)
		totalFormatted := formatSeconds(totalDur)

		display.Tag(c, fmt.Sprintf("%s / %s", curFormatted, totalFormatted), display.TagProps{
			Size:  lobetheme.Small,
			Color: display.TagBlue,
		})

		// Scrubber slider
		sliderPos := curPos
		if choice.Slider(c, &sliderPos, choice.SliderProps{
			Min: 0,
			Max: totalDur,
		}).Changed() {
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

		if action.ActionIcon(c, muteIcon, action.ActionIconProps{
			Size:  lobetheme.Small,
			Title: muteTitle,
		}).Clicked() {
			if s.videoPlayer != nil {
				s.videoPlayer.SetMuted(!isMuted)
				s.requestRedraw()
			}
		}
	})
}

func (s *Screen) togglePlayPause() {
	if s.videoPlayer == nil {
		// If video player isn't active (e.g. test or no ffmpeg), still toggle throttler for testing
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
	if s.videoDecoder != nil {
		_ = s.videoDecoder.Close()
		s.videoDecoder = nil
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
