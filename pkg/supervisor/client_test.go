package supervisor

import (
	"context"
	"encoding/json/v2"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// serve applies the Supervisor's own rule that every POST and DELETE body is a
// JSON object.
func serve(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			var body map[string]any
			if err := json.UnmarshalRead(r.Body, &body); err != nil {
				t.Errorf("%s %s: body is not a JSON object: %v", r.Method, r.URL.Path, err)
				http.Error(w, `{"result":"error","message":"Invalid json"}`, http.StatusBadRequest)
				return
			}
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("Authorization = %q", got)
		}
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	return NewClient(srv.URL+"/", "tok")
}

func reply(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		io.WriteString(w, body)
	}
}

func TestCreateBackup(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/backups/new/full" {
			t.Errorf("got %s %s", r.Method, r.URL.Path)
		}
		io.WriteString(w, `{"result":"ok","data":{"job_id":"j1","slug":"abc123"}}`)
	})
	if slug, err := c.CreateBackup(context.Background()); err != nil || slug != "abc123" {
		t.Errorf("slug, err = %q, %v", slug, err)
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		name    string
		h       http.HandlerFunc
		wantErr string
	}{
		{"forbidden", reply(403, `{"result":"error","message":"No access"}`), "hassio_role: backup"},
		{"message", reply(400, `{"result":"error","message":"boom"}`), "boom"},
		{"non-json", reply(502, `bad gateway`), "bad gateway"},
		{"no slug", reply(200, `{"result":"ok","data":{}}`), "empty slug"},
		{"error at 200", reply(200, `{"result":"error","message":"nope"}`), "nope"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := serve(t, tt.h).CreateBackup(context.Background())
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestDownloadBackup(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/backups/abc123/download" {
			t.Errorf("got %s %s", r.Method, r.URL.Path)
		}
		io.WriteString(w, "tar-bytes")
	})
	rc, err := c.DownloadBackup(context.Background(), "abc123")
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	if got, _ := io.ReadAll(rc); string(got) != "tar-bytes" {
		t.Errorf("body = %q", got)
	}

	c = serve(t, reply(404, `{"result":"error","message":"Backup does not exist"}`))
	if _, err := c.DownloadBackup(context.Background(), "nope"); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("err = %v", err)
	}
}

func TestDeleteBackup(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/backups/abc123" {
			t.Errorf("got %s %s", r.Method, r.URL.Path)
		}
		io.WriteString(w, `{"result":"ok","data":{}}`)
	})
	if err := c.DeleteBackup(context.Background(), "abc123"); err != nil {
		t.Fatal(err)
	}

	c = serve(t, reply(200, `{"result":"error","message":"in use"}`))
	if err := c.DeleteBackup(context.Background(), "abc123"); err == nil || !strings.Contains(err.Error(), "in use") {
		t.Errorf("err = %v", err)
	}
}
