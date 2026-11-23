package mobile

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"painless/internal/storage"
)

// dbDownloadTimeout bounds one VACUUM INTO snapshot for /download/db.
const dbDownloadTimeout = 2 * time.Minute

// LibraryStats is the library summary carried by /api/info. It mirrors
// the D-Stream desktop payload so companion clients can render their
// pairing screen without desktop-specific knowledge.
type LibraryStats struct {
	TotalItems  int64 `json:"total_items"`
	Images      int64 `json:"images"`
	Videos      int64 `json:"videos"`
	Albums      int64 `json:"albums"`
	Tags        int64 `json:"tags"`
	DBSizeBytes int64 `json:"db_size_bytes"`
	DBExists    bool  `json:"db_exists"`
}

// infoResponse is the /api/info payload. Field names match the original
// D-Stream desktop server byte for byte; omitted fields stay absent.
type infoResponse struct {
	Status      string        `json:"status"`
	Server      string        `json:"server"`
	DriveName   string        `json:"drive_name,omitempty"`
	DrivePath   string        `json:"drive_path,omitempty"`
	DownloadURL string        `json:"download_url"`
	ServerIP    string        `json:"server_ip,omitempty"`
	ServerPort  int           `json:"server_port"`
	Stats       *LibraryStats `json:"stats,omitempty"`
}

// dbPath is the live library database location. An explicit DBFile wins
// because the library may live outside the drive root; otherwise the
// database sits at the drive root.
func (s *Server) dbPath() string {
	s.mu.RLock()
	dbFile := s.opts.DBFile
	s.mu.RUnlock()
	if dbFile != "" {
		return dbFile
	}
	root := s.root()
	if root == "" {
		return ""
	}
	return filepath.Join(root, storage.LibraryDBFilename)
}

// handleInfo reports server identity, the active drive and library stats
// for companion pairing screens. Served at /, /api/info and /api/db/stats.
func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	resp := infoResponse{
		Status:     "online",
		Server:     "Media Library Mobile Sync Server",
		ServerPort: s.opts.Port,
	}
	if s.opts.DriveInfo != nil {
		if drive := s.opts.DriveInfo(); drive != nil {
			resp.DriveName = drive.Name
			resp.DrivePath = drive.Path
		}
	}
	if resp.DrivePath == "" {
		resp.DrivePath = s.root()
	}
	// Build the download URL from the request host so phones pairing via
	// any reachable address get a URL that works for them.
	host := r.Host
	if host == "" {
		host = s.Addr()
	}
	resp.DownloadURL = "http://" + host + "/download/db"
	if lan, err := LANAddress(s.opts.Port); err == nil {
		if u, err := url.Parse("http://" + lan); err == nil {
			resp.ServerIP = u.Hostname()
		}
	}
	if s.opts.LibraryStats != nil {
		resp.Stats = s.opts.LibraryStats()
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleDownloadDB serves the library database. GET first writes a
// consistent snapshot with VACUUM INTO (a live WAL database copied raw
// can miss committed pages), streams it with byte-range support and
// deletes it afterwards; HEAD answers from the live file because a
// snapshot would cost a full copy just to report metadata.
func (s *Server) handleDownloadDB(w http.ResponseWriter, r *http.Request) {
	path := s.dbPath()
	if path == "" {
		writeErrJSON(w, http.StatusNotFound, "Library database not found")
		return
	}
	if _, err := os.Stat(path); err != nil {
		writeErrJSON(w, http.StatusNotFound, "Library database not found")
		return
	}

	if r.Method == http.MethodHead {
		s.serveDBFile(w, r, path)
		return
	}
	if s.opts.DatabaseSnapshot == nil {
		// Without a snapshot hook the best we can do is the live file.
		s.serveDBFile(w, r, path)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), dbDownloadTimeout)
	defer cancel()
	snapshot, cleanup, err := s.opts.DatabaseSnapshot(ctx)
	if err != nil {
		writeErrJSON(w, http.StatusInternalServerError, "Could not snapshot library: "+err.Error())
		return
	}
	defer cleanup()
	s.serveDBFile(w, r, snapshot)
}

// serveDBFile streams one SQLite database file as an attachment with
// byte-range support (http.ServeContent implements both).
func (s *Server) serveDBFile(w http.ResponseWriter, r *http.Request, path string) {
	f, err := os.Open(path)
	if err != nil {
		writeErrJSON(w, http.StatusInternalServerError, "Could not open library database")
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		writeErrJSON(w, http.StatusInternalServerError, "Could not stat library database")
		return
	}
	name := filepath.Base(path)
	w.Header().Set("Content-Type", "application/x-sqlite3")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	http.ServeContent(w, r, name, info.ModTime(), f)
}

// writeErrJSON emits the JSON error shape D-Stream clients expect.
func writeErrJSON(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
