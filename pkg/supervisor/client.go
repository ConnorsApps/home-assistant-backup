// Package supervisor calls the Supervisor API directly, for running as an app.
// The Core proxy can't be used: it rejects the "hassio/" path backups download
// from. The Supervisor's /backups routes need hassio_api and hassio_role backup.
package supervisor

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Client has no client-wide timeout: a download streams for as long as the
// upload takes, and the caller's context bounds every call.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), token: token, http: &http.Client{}}
}

// response is the envelope around every Supervisor JSON reply.
type response struct {
	Result  string `json:"result"`
	Message string `json:"message"`
	Data    struct {
		Slug string `json:"slug"`
	} `json:"data"`
}

func statusError(code int, body []byte) error {
	hint := ""
	if code == http.StatusUnauthorized || code == http.StatusForbidden {
		hint = " (needs hassio_api: true and hassio_role: backup)"
	}
	msg := strings.TrimSpace(string(body))
	var r response
	if err := json.Unmarshal(body, &r); err == nil && r.Message != "" {
		msg = r.Message
	}
	if msg == "" {
		return fmt.Errorf("unexpected status %d%s", code, hint)
	}
	return fmt.Errorf("unexpected status %d%s: %s", code, hint, msg)
}

func (c *Client) do(ctx context.Context, method, path string) (*http.Response, error) {
	// The Supervisor parses every POST and DELETE body as JSON, so "{}" is
	// required even with no parameters.
	var body io.Reader
	if method != http.MethodGet {
		body = strings.NewReader("{}")
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		resp.Body.Close()
		return nil, statusError(resp.StatusCode, msg)
	}
	return resp, nil
}

func (c *Client) call(ctx context.Context, method, path string) (response, error) {
	var r response
	resp, err := c.do(ctx, method, path)
	if err != nil {
		return r, err
	}
	defer resp.Body.Close()
	if err := json.UnmarshalRead(resp.Body, &r); err != nil {
		return r, fmt.Errorf("decode response: %w", err)
	}
	if r.Result != "ok" {
		return r, fmt.Errorf("supervisor returned result %q: %s", r.Result, r.Message)
	}
	return r, nil
}

// CreateBackup blocks until the backup is made, which can take minutes.
func (c *Client) CreateBackup(ctx context.Context) (string, error) {
	r, err := c.call(ctx, http.MethodPost, "/backups/new/full")
	if err != nil {
		return "", err
	}
	if r.Data.Slug == "" {
		return "", fmt.Errorf("empty slug in supervisor response")
	}
	return r.Data.Slug, nil
}

// DownloadBackup streams the tar; the caller closes it.
func (c *Client) DownloadBackup(ctx context.Context, slug string) (io.ReadCloser, error) {
	resp, err := c.do(ctx, http.MethodGet, "/backups/"+url.PathEscape(slug)+"/download")
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (c *Client) DeleteBackup(ctx context.Context, slug string) error {
	_, err := c.call(ctx, http.MethodDelete, "/backups/"+url.PathEscape(slug))
	return err
}
