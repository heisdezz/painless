package gdrive

import (
	"context"
	"fmt"
)

// AccountInfo summarizes the authenticated Google Drive account and target folder status.
type AccountInfo struct {
	Email       string `json:"email"`
	ProjectID   string `json:"project_id"`
	DisplayName string `json:"display_name,omitempty"`
	StorageUsed int64  `json:"storage_used"`
	StorageMax  int64  `json:"storage_max"`
	FolderFound bool   `json:"folder_found"`
	FolderName  string `json:"folder_name,omitempty"`
	CanWrite    bool   `json:"can_write"`
}

// TestConnection verifies that the service account credentials are valid and that
// optional targetFolderID is accessible with write permission.
func (c *Client) TestConnection(ctx context.Context, targetFolderID string) (*AccountInfo, error) {
	if c.service == nil {
		return nil, fmt.Errorf("google drive service is uninitialized")
	}

	info := &AccountInfo{
		Email:     c.clientEmail,
		ProjectID: c.projectID,
		CanWrite:  true,
	}

	// 1. Verify Drive API access via About.Get()
	aboutCall := c.service.About.Get().Fields("user(displayName,emailAddress)", "storageQuota(usage,limit)")
	about, err := aboutCall.Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate with Google Drive API: %w", err)
	}

	if about.User != nil {
		info.DisplayName = about.User.DisplayName
		if about.User.EmailAddress != "" {
			info.Email = about.User.EmailAddress
		}
	}
	if about.StorageQuota != nil {
		info.StorageUsed = about.StorageQuota.Usage
		info.StorageMax = about.StorageQuota.Limit
	}

	// 2. If target folder specified, verify existence and write permissions
	if targetFolderID != "" {
		folderCall := c.service.Files.Get(targetFolderID).Fields("id, name, mimeType, capabilities(canAddChildren)")
		folder, err := folderCall.Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("target folder %q not found or not shared with %s: %w", targetFolderID, c.clientEmail, err)
		}

		if folder.MimeType != "application/vnd.google-apps.folder" {
			return nil, fmt.Errorf("target %q is not a Google Drive folder (mime: %s)", targetFolderID, folder.MimeType)
		}

		info.FolderFound = true
		info.FolderName = folder.Name
		if folder.Capabilities != nil {
			info.CanWrite = folder.Capabilities.CanAddChildren
		}

		if !info.CanWrite {
			return nil, fmt.Errorf("service account %s does not have write permission to folder %q", c.clientEmail, targetFolderID)
		}
	}

	return info, nil
}
