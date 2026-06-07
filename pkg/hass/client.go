package hass

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

)

func statusError(code int, body []byte) error {
	hint := ""
	switch code {
	case http.StatusUnauthorized:
		hint = " (HASS_URL must be the Home Assistant Core URL (e.g. https://homeassistant.local:8123), HASS_TOKEN must be an admin long-lived access token, and any reverse proxy in front of HA must forward the Authorization header)"
	case http.StatusForbidden:
		hint = " (token lacks required permissions)"
	case http.StatusNotFound:
		hint = " (check HASS_URL is correct)"
	}
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		return fmt.Errorf("unexpected status %d%s", code, hint)
	}
	return fmt.Errorf("unexpected status %d%s: %s", code, hint, msg)
}

type ClientOptions struct {
	InsecureSkipVerify bool
	Timeout            time.Duration
}

type Client struct {
	baseURL string
	token   string
	http    *http.Client
	tlsCfg  *tls.Config
}

func (c *Client) wsURL() string {
	url := strings.Replace(c.baseURL+"/api/websocket", "https://", "wss://", 1)
	return strings.Replace(url, "http://", "ws://", 1)
}

func NewClient(baseURL, token string, opts ClientOptions) *Client {
	tlsCfg := &tls.Config{InsecureSkipVerify: opts.InsecureSkipVerify} //nolint:gosec
	transport := &http.Transport{
		TLSClientConfig: tlsCfg,
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    &http.Client{Transport: transport, Timeout: opts.Timeout},
		tlsCfg:  tlsCfg,
	}
}

type createBackupResponse struct {
	ServiceResponse struct {
		Backup string `json:"backup"`
	} `json:"service_response"`
}

// CreateBackup triggers a full backup via the hassio.backup_full service and returns the backup slug.
// Goes through the Core service handler (not the /api/hassio/ proxy, whose PATHS_ADMIN allowlist
// rejects POST /backups/new/full with 401). Blocks until the backup is created — can take several
// minutes for large installations.
func (c *Client) CreateBackup(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/services/hassio/backup_full?return_response", bytes.NewReader([]byte("{}")))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("%w", statusError(resp.StatusCode, body))
	}

	var result createBackupResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if result.ServiceResponse.Backup == "" {
		return "", fmt.Errorf("empty slug in service response")
	}

	return result.ServiceResponse.Backup, nil
}

// DownloadBackup streams the backup tar file for the given slug.
// The caller is responsible for closing the returned ReadCloser.
func (c *Client) DownloadBackup(ctx context.Context, slug string) (io.ReadCloser, error) {
	url := fmt.Sprintf("%s/api/hassio/backups/%s/download", c.baseURL, slug)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		resp.Body.Close()
		return nil, statusError(resp.StatusCode, body)
	}

	return resp.Body, nil
}
