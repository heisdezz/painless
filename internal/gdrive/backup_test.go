package gdrive

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

type mockSnapshotter struct {
	called     bool
	backupPath string
	content    []byte
}

func (m *mockSnapshotter) CreateBackup(backupPath string) error {
	m.called = true
	m.backupPath = backupPath
	return os.WriteFile(backupPath, m.content, 0644)
}

func TestValidateCredentialsJSON(t *testing.T) {
	// 1. Empty data
	if _, err := ValidateCredentialsJSON(nil); err == nil {
		t.Errorf("expected error for empty credentials")
	}

	// 2. Invalid JSON
	if _, err := ValidateCredentialsJSON([]byte("{bad-json")); err == nil {
		t.Errorf("expected error for malformed JSON")
	}

	// 3. Wrong type
	wrongType := []byte(`{"type": "user_account"}`)
	if _, err := ValidateCredentialsJSON(wrongType); err == nil {
		t.Errorf("expected error for non-service_account type")
	}

	// 4. Missing fields
	incomplete := []byte(`{"type": "service_account", "client_email": "foo@example.com"}`)
	if _, err := ValidateCredentialsJSON(incomplete); err == nil {
		t.Errorf("expected error for missing private_key")
	}

	// 5. Valid service account
	valid := []byte(`{
		"type": "service_account",
		"project_id": "test-project",
		"private_key_id": "key-123",
		"private_key": "-----BEGIN PRIVATE KEY-----\nMIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQC...\n-----END PRIVATE KEY-----\n",
		"client_email": "sa@test-project.iam.gserviceaccount.com"
	}`)
	key, err := ValidateCredentialsJSON(valid)
	if err != nil {
		t.Fatalf("expected valid JSON to parse: %v", err)
	}
	if key.ClientEmail != "sa@test-project.iam.gserviceaccount.com" {
		t.Errorf("unexpected client email: %s", key.ClientEmail)
	}
	if key.ProjectID != "test-project" {
		t.Errorf("unexpected project id: %s", key.ProjectID)
	}
}

func setupMockDriveServer(t *testing.T) (*httptest.Server, *Client) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// About endpoint
		if strings.Contains(r.URL.Path, "about") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"user": map[string]any{
					"displayName":  "Painless Backup Service",
					"emailAddress": "sa@test-project.iam.gserviceaccount.com",
				},
				"storageQuota": map[string]any{
					"usage": "2048",
					"limit": "10737418240",
				},
			})
			return
		}

		// Files Get endpoint
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "folder123") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":       "folder123",
				"name":     "Painless Cloud Backups",
				"mimeType": "application/vnd.google-apps.folder",
				"capabilities": map[string]any{
					"canAddChildren": true,
				},
			})
			return
		}
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "readonly_folder") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":       "readonly_folder",
				"name":     "Read-only Backups",
				"mimeType": "application/vnd.google-apps.folder",
				"capabilities": map[string]any{
					"canAddChildren": false,
				},
			})
			return
		}

		// Upload / Create file endpoint (POST)
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "files") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":          "uploaded-file-789",
				"name":        "media_library_backup.db",
				"webViewLink": "https://drive.google.com/file/d/uploaded-file-789/view",
				"size":        "1024",
			})
			return
		}

		http.NotFound(w, r)
	})

	server := httptest.NewServer(handler)

	ctx := context.Background()
	srv, err := drive.NewService(ctx,
		option.WithoutAuthentication(),
		option.WithEndpoint(server.URL+"/"),
		option.WithHTTPClient(server.Client()),
	)
	if err != nil {
		t.Fatalf("failed creating mock drive service: %v", err)
	}

	client := &Client{
		service:     srv,
		clientEmail: "sa@test-project.iam.gserviceaccount.com",
		projectID:   "test-project",
	}

	return server, client
}

func TestTestConnection(t *testing.T) {
	server, client := setupMockDriveServer(t)
	defer server.Close()

	ctx := context.Background()

	// 1. Success without folder
	info, err := client.TestConnection(ctx, "")
	if err != nil {
		t.Fatalf("TestConnection failed: %v", err)
	}
	if info.Email != "sa@test-project.iam.gserviceaccount.com" {
		t.Errorf("unexpected email: %s", info.Email)
	}
	if info.DisplayName != "Painless Backup Service" {
		t.Errorf("unexpected display name: %s", info.DisplayName)
	}
	if info.StorageUsed != 2048 {
		t.Errorf("unexpected storage used: %d", info.StorageUsed)
	}

	// 2. Success with valid writable folder
	info, err = client.TestConnection(ctx, "folder123")
	if err != nil {
		t.Fatalf("TestConnection with folder failed: %v", err)
	}
	if !info.FolderFound || !info.CanWrite || info.FolderName != "Painless Cloud Backups" {
		t.Errorf("folder metadata mismatch: %+v", info)
	}

	// 3. Fails when folder is read-only
	_, err = client.TestConnection(ctx, "readonly_folder")
	if err == nil {
		t.Errorf("expected error for read-only folder")
	}

	// 4. Fails for non-existent folder
	_, err = client.TestConnection(ctx, "non_existent")
	if err == nil {
		t.Errorf("expected error for non-existent folder")
	}
}

func TestBackupDatabase_SuccessAndCleanup(t *testing.T) {
	server, client := setupMockDriveServer(t)
	defer server.Close()

	tempDir := t.TempDir()
	snapshotter := &mockSnapshotter{
		content: []byte("SQLite format 3\x00 fake sqlite db file content"),
	}

	progressReported := false
	var totalReported int64

	res, err := client.BackupDatabase(context.Background(), snapshotter, "folder123", tempDir, func(written, total int64) {
		progressReported = true
		totalReported = total
	})

	if err != nil {
		t.Fatalf("BackupDatabase failed: %v", err)
	}
	if res.FileID != "uploaded-file-789" {
		t.Errorf("unexpected file ID: %s", res.FileID)
	}
	if res.Size != 1024 {
		t.Errorf("unexpected file size: %d", res.Size)
	}
	if !progressReported || totalReported != int64(len(snapshotter.content)) {
		t.Errorf("progress was not reported accurately: reported=%v, total=%d", progressReported, totalReported)
	}

	// Verify temporary backup file was deleted after upload
	if snapshotter.backupPath == "" {
		t.Errorf("snapshotter was never called")
	}
	if _, err := os.Stat(snapshotter.backupPath); !os.IsNotExist(err) {
		t.Errorf("temporary snapshot file %s should have been removed after upload", snapshotter.backupPath)
	}
}

func TestBackupDatabase_ContextCancellation(t *testing.T) {
	server, client := setupMockDriveServer(t)
	defer server.Close()

	tempDir := t.TempDir()
	snapshotter := &mockSnapshotter{
		content: []byte("sample db content"),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.BackupDatabase(ctx, snapshotter, "folder123", tempDir, nil)
	if err == nil {
		t.Errorf("expected cancellation error")
	}
}
