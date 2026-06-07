package hass

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type ClientOptions struct {
	InsecureSkipVerify bool
	Timeout            time.Duration
}

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func NewClient(baseURL, token string, opts ClientOptions) *Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: opts.InsecureSkipVerify}, //nolint:gosec
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    &http.Client{Transport: transport, Timeout: opts.Timeout},
	}
}

type createBackupResponse struct {
	Result string `json:"result"`
	Data   struct {
		Slug string `json:"slug"`
	} `json:"data"`
}

// CreateBackup triggers a full backup via the HA Supervisor API and returns the backup slug.
// This call blocks until the backup is created (can take several minutes for large installations).
func (c *Client) CreateBackup(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/hassio/backups/new/full", nil)
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
		return "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var result createBackupResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if result.Result != "ok" {
		return "", fmt.Errorf("backup failed: result=%q", result.Result)
	}
	if result.Data.Slug == "" {
		return "", fmt.Errorf("empty slug in response")
	}

	return result.Data.Slug, nil
}

// DeleteBackup removes the backup with the given slug from Home Assistant.
func (c *Client) DeleteBackup(ctx context.Context, slug string) error {
	url := fmt.Sprintf("%s/api/hassio/backups/%s", c.baseURL, slug)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}
	return nil
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
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	return resp.Body, nil
}
