package viewer

import (
	"fmt"
	"path/filepath"
	"strings"

	mui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/layout"
	lobetheme "github.com/lobehub/lobe-mygo/pkg/theme"

	"painless/internal/domain"
)

// infoRailWidth is the fixed width of the inspector rail next to the stage.
const infoRailWidth = 300

func (s *Screen) infoRail(c *mui.Context, tk *lobetheme.Tokens, item domain.MediaItem) {
	meta := item.ParsedMetadata
	if meta == nil {
		meta, _ = item.ParseMetadata()
	}

	mui.Column(c).Width(infoRailWidth).Shrink(0).AlignSelf(mui.Stretch).
		Background(tk.BgLayout).Border(1, tk.BorderSecondary).Children(func() {
		mui.Scroll(c).Fill().Children(func() {
			mui.Column(c).Fill().Padding(16, 16, 24).Children(func() {
				mui.Text(c, "Media Details & EXIF").
					FontSize(11).FontWeight(600).TextColor(tk.TextTertiary).LetterSpacing(0.5)
				mui.Row(c).Padding(6, 0, 14).AlignItems(mui.Center).Gap(8).Children(func() {
					display.Tag(c, formatTag(item), display.TagProps{
						Size:  lobetheme.Small,
						Color: display.TagBlue,
					})
					mui.Text(c, formatBytes(uint64(item.FileSize))).
						FontSize(12).TextColor(tk.TextSecondary).SingleLine()
				})

				railSection(c, tk, "File Attributes", func() {
					kvRow(c, tk, "Filename", filepath.Base(item.CurrentRelativePath))
					kvRow(c, tk, "File Size", formatBytes(uint64(item.FileSize)))
					kvRow(c, tk, "MIME Type", item.MimeType)
					if !item.CreatedAt.IsZero() {
						kvRow(c, tk, "Discovered", item.CreatedAt.Format("2006-01-02 15:04"))
					}
					if item.AlbumID != nil {
						kvRow(c, tk, "Album", fmt.Sprintf("#%d", *item.AlbumID))
					}
					if item.OriginalRelativePath != "" && item.OriginalRelativePath != item.CurrentRelativePath {
						kvBlock(c, tk, "Original Location", item.OriginalRelativePath, false)
					}
					kvBlock(c, tk, "Active Catalog Path", item.CurrentRelativePath, false)
					kvBlock(c, tk, "SHA-256", item.FileHash, true)
				})

				layout.Divider(c, layout.DividerProps{})

				railSection(c, tk, "Dimensions & Format", func() {
					if meta != nil && meta.Width > 0 && meta.Height > 0 {
						gcd := calculateGCD(meta.Width, meta.Height)
						kvRow(c, tk, "Resolution", fmt.Sprintf("%d × %d", meta.Width, meta.Height))
						kvRow(c, tk, "Aspect Ratio", fmt.Sprintf("%d:%d", meta.Width/gcd, meta.Height/gcd))
					} else {
						kvRow(c, tk, "Resolution", "Unknown")
					}
					if item.DurationSeconds != nil && *item.DurationSeconds > 0 {
						kvRow(c, tk, "Duration", formatSeconds(float64(*item.DurationSeconds)))
					}
					if meta != nil && meta.VideoCodec != "" {
						kvRow(c, tk, "Video Codec", meta.VideoCodec)
					}
					if meta != nil && meta.AudioCodec != "" {
						kvRow(c, tk, "Audio Codec", meta.AudioCodec)
					}
					if meta != nil && meta.Bitrate > 0 {
						kvRow(c, tk, "Bitrate", fmt.Sprintf("%d kbps", meta.Bitrate/1000))
					}
				})

				layout.Divider(c, layout.DividerProps{})

				railSection(c, tk, "EXIF Camera Details", func() {
					s.renderCameraDetails(c, tk, meta)
				})

				layout.Divider(c, layout.DividerProps{})

				railSection(c, tk, "GPS Location", func() {
					s.renderGPS(c, tk, meta)
				})
			})
		})
	})
}

// railSection renders one inspector group: a micro heading over a tight
// column of rows, separated from its neighbors by hairline dividers.
func railSection(c *mui.Context, tk *lobetheme.Tokens, title string, body func()) {
	mui.Column(c).Gap(8).Padding(0, 0, 14).Children(func() {
		mui.Text(c, title).FontSize(11).FontWeight(600).TextColor(tk.TextTertiary)
		body()
	})
}

// kvRow is a compact inspector row: label left, value right-aligned.
func kvRow(c *mui.Context, tk *lobetheme.Tokens, key, val string) {
	if val == "" {
		val = "—"
	}
	mui.Row(c).AlignItems(mui.Center).Gap(12).Children(func() {
		mui.Text(c, key).FontSize(12).TextColor(tk.TextSecondary).Shrink(0)
		mui.Text(c, val).FontSize(12).TextColor(tk.Text).Grow(1).
			SingleLine().TextAlign(mui.End)
	})
}

// kvBlock stacks a long value under its label so paths and hashes wrap
// instead of truncating.
func kvBlock(c *mui.Context, tk *lobetheme.Tokens, key, val string, allowCopy bool) {
	if val == "" {
		return
	}
	mui.Column(c).Gap(2).Children(func() {
		mui.Text(c, key).FontSize(12).TextColor(tk.TextSecondary).SingleLine()
		mui.Row(c).AlignItems(mui.Center).Gap(6).Children(func() {
			mui.Text(c, val).FontSize(12).TextColor(tk.Text).Grow(1)
			if allowCopy {
				action.CopyButton(c, val, action.CopyButtonProps{Size: lobetheme.Small})
			}
		})
	})
}

// formatTag renders the container format (MP4, JPEG...) from the MIME type.
func formatTag(item domain.MediaItem) string {
	subtype := item.MimeType
	if slash := strings.Index(subtype, "/"); slash >= 0 {
		subtype = subtype[slash+1:]
	}
	if subtype == "" {
		return "File"
	}
	return strings.ToUpper(subtype)
}

func (s *Screen) renderCameraDetails(c *mui.Context, tk *lobetheme.Tokens, meta *domain.MediaMetadata) {
	if meta == nil || (meta.CameraMake == "" && meta.CameraModel == "") {
		mui.Text(c, "No EXIF camera metadata recorded.").
			FontSize(12).TextColor(tk.TextTertiary)
		return
	}

	if meta.CameraMake != "" {
		kvRow(c, tk, "Make", meta.CameraMake)
	}
	if meta.CameraModel != "" {
		kvRow(c, tk, "Model", meta.CameraModel)
	}
	if meta.LensModel != "" {
		kvRow(c, tk, "Lens", meta.LensModel)
	}
	if meta.Aperture > 0 {
		kvRow(c, tk, "Aperture", fmt.Sprintf("f/%.1f", meta.Aperture))
	}
	if meta.ExposureTime != "" {
		kvRow(c, tk, "Exposure", meta.ExposureTime)
	}
	if meta.ISO > 0 {
		kvRow(c, tk, "ISO", fmt.Sprintf("%d", meta.ISO))
	}
	if meta.FocalLength > 0 {
		kvRow(c, tk, "Focal Length", fmt.Sprintf("%.1f mm", meta.FocalLength))
	}
	if meta.DateTaken != nil {
		kvRow(c, tk, "Date Taken", meta.DateTaken.Format("2006-01-02 15:04:05"))
	}
}

func (s *Screen) renderGPS(c *mui.Context, tk *lobetheme.Tokens, meta *domain.MediaMetadata) {
	if meta == nil || meta.GPSLatitude == nil || meta.GPSLongitude == nil {
		mui.Text(c, "No GPS location coordinates available.").
			FontSize(12).TextColor(tk.TextTertiary)
		return
	}

	latStr := fmt.Sprintf("%.6f", *meta.GPSLatitude)
	lonStr := fmt.Sprintf("%.6f", *meta.GPSLongitude)

	kvRow(c, tk, "Latitude", latStr)
	kvRow(c, tk, "Longitude", lonStr)
	if meta.GPSAltitude != nil {
		kvRow(c, tk, "Altitude", fmt.Sprintf("%.1f m", *meta.GPSAltitude))
	}

	coordsStr := latStr + ", " + lonStr
	mui.Row(c).Padding(2, 0, 0).AlignItems(mui.Center).Gap(8).Children(func() {
		mui.Text(c, coordsStr).FontSize(12).FontWeight(500).TextColor(tk.Text).Grow(1).SingleLine()
		action.CopyButton(c, coordsStr, action.CopyButtonProps{Size: lobetheme.Small})
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
