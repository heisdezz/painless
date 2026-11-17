package gdrive

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// ServiceAccountKey represents standard fields in a Google Cloud service account JSON file.
type ServiceAccountKey struct {
	Type                    string `json:"type"`
	ProjectID               string `json:"project_id"`
	PrivateKeyID            string `json:"private_key_id"`
	PrivateKey              string `json:"private_key"`
	ClientEmail             string `json:"client_email"`
	ClientID                string `json:"client_id"`
	AuthURI                 string `json:"auth_uri"`
	TokenURI                string `json:"token_uri"`
	AuthProviderX509CertURL string `json:"auth_provider_x509_cert_url"`
	ClientX509CertURL       string `json:"client_x509_cert_url"`
}

// Client wraps the official Google Drive v3 API client.
type Client struct {
	service     *drive.Service
	clientEmail string
	projectID   string
}

// ValidateCredentialsJSON checks whether the provided JSON data is a well-formed Google Service Account key.
func ValidateCredentialsJSON(data []byte) (*ServiceAccountKey, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("credentials JSON is empty")
	}

	var key ServiceAccountKey
	if err := json.Unmarshal(data, &key); err != nil {
		return nil, fmt.Errorf("invalid service account JSON structure: %w", err)
	}

	if key.Type != "service_account" {
		return nil, fmt.Errorf("unsupported credential type %q (expected \"service_account\")", key.Type)
	}
	if key.ClientEmail == "" {
		return nil, fmt.Errorf("missing \"client_email\" in service account credentials")
	}
	if key.PrivateKey == "" {
		return nil, fmt.Errorf("missing \"private_key\" in service account credentials")
	}
	if key.ProjectID == "" {
		return nil, fmt.Errorf("missing \"project_id\" in service account credentials")
	}

	return &key, nil
}

// ClientOptions allows custom overrides, e.g. for testing with custom HTTP clients.
type ClientOptions struct {
	HTTPClient *http.Client
	Endpoint   string
}

// NewClient initializes an authenticated Google Drive client using service account JSON credentials.
func NewClient(ctx context.Context, credentialsJSON []byte, opts ...ClientOptions) (*Client, error) {
	key, err := ValidateCredentialsJSON(credentialsJSON)
	if err != nil {
		return nil, err
	}

	driveOptions := []option.ClientOption{
		option.WithCredentialsJSON(credentialsJSON),
		option.WithScopes(drive.DriveFileScope, drive.DriveMetadataReadonlyScope),
	}

	if len(opts) > 0 {
		if opts[0].HTTPClient != nil {
			driveOptions = append(driveOptions, option.WithHTTPClient(opts[0].HTTPClient))
		}
		if opts[0].Endpoint != "" {
			driveOptions = append(driveOptions, option.WithEndpoint(opts[0].Endpoint))
		}
	}

	srv, err := drive.NewService(ctx, driveOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Google Drive service: %w", err)
	}

	return &Client{
		service:     srv,
		clientEmail: key.ClientEmail,
		projectID:   key.ProjectID,
	}, nil
}

// ClientEmail returns the service account's client email.
func (c *Client) ClientEmail() string {
	return c.clientEmail
}

// ProjectID returns the Google Cloud project ID.
func (c *Client) ProjectID() string {
	return c.projectID
}

// Service returns the underlying drive.Service instance.
func (c *Client) Service() *drive.Service {
	return c.service
}
