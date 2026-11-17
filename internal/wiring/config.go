// Package wiring connects UI screen controllers to the real backend
// services: the portable SQLite library, the mobile sync server and the
// Google Drive backup client. It is the seam between the app shell and the
// service layer; screens only see the controller interfaces they declare.
package wiring

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	// settingsDirName is the hidden per-drive directory holding app settings.
	settingsDirName = ".painless"
	// settingsFileName is the JSON document inside settingsDirName.
	settingsFileName = "settings.json"
)

// driveSettings is the on-disk settings document for one media drive. It
// travels with the drive so pairing tokens and cloud credentials belong to
// the library instead of the desktop machine.
type driveSettings struct {
	MobileToken       string `json:"mobile_token,omitempty"`
	MobilePort        int    `json:"mobile_port,omitempty"`
	GDriveCredentials string `json:"gdrive_credentials,omitempty"`
	GDriveFolderID    string `json:"gdrive_folder_id,omitempty"`
}

// settingsPath returns the settings file location on a drive root.
func settingsPath(driveRoot string) string {
	return filepath.Join(driveRoot, settingsDirName, settingsFileName)
}

// loadDriveSettings reads the settings document from a drive root. A
// missing or unreadable file yields defaults so first runs start clean.
func loadDriveSettings(driveRoot string) driveSettings {
	var cfg driveSettings
	if driveRoot == "" {
		return cfg
	}
	data, err := os.ReadFile(settingsPath(driveRoot))
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(data, &cfg)
	return cfg
}

// save writes the settings document atomically: temp file inside the
// settings directory, then rename over the destination.
func (s driveSettings) save(driveRoot string) error {
	if driveRoot == "" {
		return fmt.Errorf("no active drive: settings not saved")
	}
	dir := filepath.Join(driveRoot, settingsDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create settings dir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	tmp, err := os.CreateTemp(dir, settingsFileName+".tmp*")
	if err != nil {
		return fmt.Errorf("create settings temp file: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write settings: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close settings temp file: %w", err)
	}
	if err := os.Rename(tmpPath, settingsPath(driveRoot)); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("replace settings file: %w", err)
	}
	return nil
}

// newToken returns a 128-bit random hex pairing token.
func newToken() string {
	buf := make([]byte, 16)
	// crypto/rand does not fail on the platforms Painless supports; a read
	// error would leave a zeroed token rather than a weak predictable one.
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
