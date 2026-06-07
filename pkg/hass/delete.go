package hass

import (
	"context"
	"fmt"

	"golang.org/x/net/websocket"
)

type wsMsg = map[string]any

// DeleteBackup removes the backup with the given slug from Home Assistant via the
// WebSocket API (backup/delete command). The REST proxy at /api/hassio/backups/{slug}
// only registers GET and POST handlers, so DELETE returns 405; there is also no
// hassio.backup_remove service. The WebSocket path is the only officially supported
// route for deletion in HA 2025.1+.
func (c *Client) DeleteBackup(ctx context.Context, slug string) error {
	cfg, err := websocket.NewConfig(c.wsURL(), c.baseURL+"/")
	if err != nil {
		return fmt.Errorf("build websocket config: %w", err)
	}
	cfg.TlsConfig = c.tlsCfg

	ws, err := websocket.DialConfig(cfg)
	if err != nil {
		return fmt.Errorf("websocket dial: %w", err)
	}
	defer ws.Close()

	// Close the WebSocket if the context is cancelled.
	connClosed := make(chan struct{})
	defer close(connClosed)
	go func() {
		select {
		case <-ctx.Done():
			ws.Close()
		case <-connClosed:
		}
	}()

	recv := func() (wsMsg, error) {
		var m wsMsg
		if err := websocket.JSON.Receive(ws, &m); err != nil {
			return nil, err
		}
		return m, nil
	}
	send := func(v any) error {
		return websocket.JSON.Send(ws, v)
	}

	// 1. Wait for auth_required.
	msg, err := recv()
	if err != nil {
		return fmt.Errorf("read auth_required: %w", err)
	}
	if msg["type"] != "auth_required" {
		return fmt.Errorf("expected auth_required, got %v", msg["type"])
	}

	// 2. Authenticate.
	if err := send(wsMsg{"type": "auth", "access_token": c.token}); err != nil {
		return fmt.Errorf("send auth: %w", err)
	}

	// 3. Wait for auth_ok.
	msg, err = recv()
	if err != nil {
		return fmt.Errorf("read auth response: %w", err)
	}
	if msg["type"] != "auth_ok" {
		return fmt.Errorf("authentication failed: %v", msg["message"])
	}

	// 4. Send backup/delete command. The backup_id for supervisor backups is the slug.
	if err := send(wsMsg{"id": 1, "type": "backup/delete", "backup_id": slug}); err != nil {
		return fmt.Errorf("send backup/delete: %w", err)
	}

	// 5. Read command result.
	msg, err = recv()
	if err != nil {
		return fmt.Errorf("read result: %w", err)
	}
	if msg["type"] != "result" {
		return fmt.Errorf("unexpected message type: %v", msg["type"])
	}
	if success, _ := msg["success"].(bool); !success {
		return fmt.Errorf("backup delete failed: %v", msg["error"])
	}

	if result, ok := msg["result"].(map[string]any); ok {
		if agentErrors, ok := result["agent_errors"].(map[string]any); ok && len(agentErrors) > 0 {
			return fmt.Errorf("backup delete had agent errors: %v", agentErrors)
		}
	}

	return nil
}
