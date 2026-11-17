package viewer

import (
	"fmt"
	"path/filepath"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/domain"
)

func (s *Screen) metadataDrawer(c *mui.Context, tk *lobetheme.Tokens) {
	if !s.drawerOpen || s.currentItem == nil {
		return
	}

	feedback.Drawer(c, &s.drawerOpen, feedback.DrawerProps{
		Placement: feedback.Right,
		Title:     "Media Details & EXIF",
		Size:      400,
	}, func() {
		item := s.currentItem
		meta := item.ParsedMetadata
		if meta == nil {
			meta, _ = item.ParseMetadata()
		}

		items := []display.AccordionItem{
			{Key: "file", Title: "File Attributes"},
			{Key: "dimensions", Title: "Dimensions & Format"},
			{Key: "camera", Title: "EXIF Camera Details"},
			{Key: "gps", Title: "GPS Location"},
		}

		mui.Scroll(c).Fill().Padding(12, 16).Children(func() {
			display.Accordion(c, s.accordionOpen, display.AccordionProps{
				Items:   items,
				Variant: display.AccordionOutlined,
			}, func(key string) {
				switch key {
				case "file":
					s.renderFileAttributes(c, tk, item)
				case "dimensions":
					s.renderDimensions(c, tk, item, meta)
				case "camera":
					s.renderCameraDetails(c, tk, meta)
				case "gps":
					s.renderGPS(c, tk, meta)
				}
			})
		})
	})
}

func (s *Screen) renderFileAttributes(c *mui.Context, tk *lobetheme.Tokens, item *domain.MediaItem) {
	mui.Column(c).Padding(8, 0).Gap(8).Children(func() {
		fileName := filepath.Base(item.CurrentRelativePath)
		s.keyValueRow(c, tk, "Filename", fileName, false)
		s.keyValueRow(c, tk, "Relative Path", item.CurrentRelativePath, false)
		s.keyValueRow(c, tk, "File Size", formatBytes(uint64(item.FileSize)), false)
		s.keyValueRow(c, tk, "MIME Type", item.MimeType, false)
		s.keyValueRow(c, tk, "SHA-256", item.FileHash, true)
	})
}

func (s *Screen) renderDimensions(c *mui.Context, tk *lobetheme.Tokens, item *domain.MediaItem, meta *domain.MediaMetadata) {
	mui.Column(c).Padding(8, 0).Gap(8).Children(func() {
		if meta != nil && meta.Width > 0 && meta.Height > 0 {
			resStr := fmt.Sprintf("%d × %d", meta.Width, meta.Height)
			s.keyValueRow(c, tk, "Resolution", resStr, false)

			gcd := calculateGCD(meta.Width, meta.Height)
			aspectStr := fmt.Sprintf("%d:%d", meta.Width/gcd, meta.Height/gcd)
			s.keyValueRow(c, tk, "Aspect Ratio", aspectStr, false)
		} else {
			s.keyValueRow(c, tk, "Resolution", "Unknown", false)
		}

		if item.DurationSeconds != nil && *item.DurationSeconds > 0 {
			mins := *item.DurationSeconds / 60
			secs := *item.DurationSeconds % 60
			s.keyValueRow(c, tk, "Duration", fmt.Sprintf("%d:%02d", mins, secs), false)
		}

		if meta != nil && meta.VideoCodec != "" {
			s.keyValueRow(c, tk, "Video Codec", meta.VideoCodec, false)
		}
		if meta != nil && meta.AudioCodec != "" {
			s.keyValueRow(c, tk, "Audio Codec", meta.AudioCodec, false)
		}
		if meta != nil && meta.Bitrate > 0 {
			bitrateKbps := meta.Bitrate / 1000
			s.keyValueRow(c, tk, "Bitrate", fmt.Sprintf("%d kbps", bitrateKbps), false)
		}
	})
}

func (s *Screen) renderCameraDetails(c *mui.Context, tk *lobetheme.Tokens, meta *domain.MediaMetadata) {
	mui.Column(c).Padding(8, 0).Gap(8).Children(func() {
		if meta == nil || (meta.CameraMake == "" && meta.CameraModel == "") {
			mui.Text(c, "No EXIF camera metadata recorded.").
				FontSize(12).TextColor(tk.TextTertiary).Padding(4, 0)
			return
		}

		if meta.CameraMake != "" {
			s.keyValueRow(c, tk, "Make", meta.CameraMake, false)
		}
		if meta.CameraModel != "" {
			s.keyValueRow(c, tk, "Model", meta.CameraModel, false)
		}
		if meta.LensModel != "" {
			s.keyValueRow(c, tk, "Lens", meta.LensModel, false)
		}
		if meta.Aperture > 0 {
			s.keyValueRow(c, tk, "Aperture", fmt.Sprintf("f/%.1f", meta.Aperture), false)
		}
		if meta.ExposureTime != "" {
			s.keyValueRow(c, tk, "Exposure", meta.ExposureTime, false)
		}
		if meta.ISO > 0 {
			s.keyValueRow(c, tk, "ISO", fmt.Sprintf("%d", meta.ISO), false)
		}
		if meta.FocalLength > 0 {
			s.keyValueRow(c, tk, "Focal Length", fmt.Sprintf("%.1f mm", meta.FocalLength), false)
		}
		if meta.DateTaken != nil {
			s.keyValueRow(c, tk, "Date Taken", meta.DateTaken.Format("2006-01-02 15:04:05"), false)
		}
	})
}

func (s *Screen) renderGPS(c *mui.Context, tk *lobetheme.Tokens, meta *domain.MediaMetadata) {
	mui.Column(c).Padding(8, 0).Gap(8).Children(func() {
		if meta == nil || meta.GPSLatitude == nil || meta.GPSLongitude == nil {
			mui.Text(c, "No GPS location coordinates available.").
				FontSize(12).TextColor(tk.TextTertiary).Padding(4, 0)
			return
		}

		latStr := fmt.Sprintf("%.6f", *meta.GPSLatitude)
		lonStr := fmt.Sprintf("%.6f", *meta.GPSLongitude)
		coordsStr := fmt.Sprintf("%s, %s", latStr, lonStr)

		s.keyValueRow(c, tk, "Latitude", latStr, false)
		s.keyValueRow(c, tk, "Longitude", lonStr, false)
		if meta.GPSAltitude != nil {
			s.keyValueRow(c, tk, "Altitude", fmt.Sprintf("%.1f m", *meta.GPSAltitude), false)
		}

		mui.Row(c).Padding(4, 0).AlignItems(mui.Center).Gap(8).Children(func() {
			mui.Text(c, "Coordinates:").FontSize(12).TextColor(tk.TextSecondary)
			mui.Text(c, coordsStr).FontSize(12).FontWeight(600).Grow(1)
			action.CopyButton(c, coordsStr, action.CopyButtonProps{
				Size: lobetheme.Small,
			})
		})
	})
}

func (s *Screen) keyValueRow(c *mui.Context, tk *lobetheme.Tokens, key, val string, allowCopy bool) {
	mui.Row(c).AlignItems(mui.Center).Children(func() {
		mui.Text(c, key).Width(100).FontSize(12).TextColor(tk.TextSecondary)
		mui.Text(c, val).FontSize(12).FontWeight(500).TextColor(tk.Text).Grow(1).SingleLine()
		if allowCopy && val != "" {
			action.CopyButton(c, val, action.CopyButtonProps{
				Size: lobetheme.Small,
			})
		}
	})
}

func calculateGCD(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	if a < 1 {
		return 1
	}
	return a
}

func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
