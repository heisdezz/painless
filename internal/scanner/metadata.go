package scanner

import (
	"encoding/binary"
	"encoding/json"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"strings"

	"painless/internal/domain"

	"github.com/rwcarlsen/goexif/exif"
)

// ExtractMetadata attempts to extract dimensions, EXIF tags, and container duration.
func ExtractMetadata(filePath string, mimeType string) (domain.MediaMetadata, *int, string) {
	meta := domain.MediaMetadata{}
	var durationSeconds *int

	// 1. Image metadata & EXIF
	if strings.HasPrefix(mimeType, "image/") {
		// Read dimensions using standard library image config decoder
		if f, err := os.Open(filePath); err == nil {
			if cfg, _, err := image.DecodeConfig(f); err == nil {
				meta.Width = cfg.Width
				meta.Height = cfg.Height
			}
			f.Close()
		}

		// Read EXIF tags if JPEG / TIFF
		if strings.Contains(mimeType, "jpeg") || strings.Contains(mimeType, "tiff") {
			if f, err := os.Open(filePath); err == nil {
				defer f.Close()
				if x, err := exif.Decode(f); err == nil {
					// Date Taken
					if dt, err := x.DateTime(); err == nil {
						meta.DateTaken = &dt
					}

					// Camera Make & Model
					if makeTag, err := x.Get(exif.Make); err == nil {
						meta.CameraMake = strings.Trim(makeTag.String(), "\"")
					}
					if modelTag, err := x.Get(exif.Model); err == nil {
						meta.CameraModel = strings.Trim(modelTag.String(), "\"")
					}

					// Exposure & ISO
					if isoTag, err := x.Get(exif.ISOSpeedRatings); err == nil {
						if isoVal, err := isoTag.Int(0); err == nil {
							meta.ISO = isoVal
						}
					}
					if expTag, err := x.Get(exif.ExposureTime); err == nil {
						meta.ExposureTime = strings.Trim(expTag.String(), "\"")
					}

					// GPS
					lat, lon, err := x.LatLong()
					if err == nil {
						meta.GPSLatitude = &lat
						meta.GPSLongitude = &lon
					}
				}
			}
		}
	}

	// 2. Video duration parsing (pure Go MP4 / QuickTime atom parser)
	if strings.HasPrefix(mimeType, "video/") {
		dur, width, height, err := parseMP4Metadata(filePath)
		if err == nil {
			if dur > 0 {
				durInt := int(dur)
				durationSeconds = &durInt
				meta.DurationSeconds = &dur
			}
			if width > 0 && meta.Width == 0 {
				meta.Width = width
			}
			if height > 0 && meta.Height == 0 {
				meta.Height = height
			}
		}
	}

	jsonBytes, err := json.Marshal(meta)
	if err != nil {
		return meta, durationSeconds, "{}"
	}

	return meta, durationSeconds, string(jsonBytes)
}

// parseMP4Metadata reads the mvhd atom from an MP4/MOV file to calculate duration and resolution.
func parseMP4Metadata(filePath string) (float64, int, int, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return 0, 0, 0, err
	}
	defer f.Close()

	// Read atoms sequentially
	var buf [8]byte
	for {
		_, err := io.ReadFull(f, buf[:])
		if err != nil {
			break
		}

		atomSize := binary.BigEndian.Uint32(buf[0:4])
		atomType := string(buf[4:8])

		if atomSize == 0 {
			break
		}

		if atomType == "moov" {
			// Descend into moov atom
			return parseMoovAtom(f, int64(atomSize)-8)
		}

		// Skip atom payload
		skip := int64(atomSize) - 8
		if skip < 0 {
			break
		}
		if _, err := f.Seek(skip, io.SeekCurrent); err != nil {
			break
		}
	}

	return 0, 0, 0, nil
}

func parseMoovAtom(r io.ReadSeeker, remaining int64) (float64, int, int, error) {
	var buf [8]byte
	var duration float64
	var width, height int

	for remaining > 8 {
		if _, err := io.ReadFull(r, buf[:]); err != nil {
			break
		}
		remaining -= 8

		atomSize := binary.BigEndian.Uint32(buf[0:4])
		atomType := string(buf[4:8])

		if atomSize < 8 || int64(atomSize) > remaining+8 {
			break
		}

		contentSize := int64(atomSize) - 8
		remaining -= contentSize

		if atomType == "mvhd" {
			// Read mvhd payload
			mvhdData := make([]byte, contentSize)
			if _, err := io.ReadFull(r, mvhdData); err == nil && len(mvhdData) >= 24 {
				version := mvhdData[0]
				var timescale uint32
				var dur uint64

				if version == 0 {
					timescale = binary.BigEndian.Uint32(mvhdData[12:16])
					dur = uint64(binary.BigEndian.Uint32(mvhdData[16:20]))
				} else if version == 1 && len(mvhdData) >= 32 {
					timescale = binary.BigEndian.Uint32(mvhdData[20:24])
					dur = binary.BigEndian.Uint64(mvhdData[24:32])
				}

				if timescale > 0 {
					duration = float64(dur) / float64(timescale)
				}
			}
		} else {
			// Seek past other atoms in moov
			if _, err := r.Seek(contentSize, io.SeekCurrent); err != nil {
				break
			}
		}
	}

	return duration, width, height, nil
}
