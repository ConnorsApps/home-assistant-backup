//go:build file || (!s3 && !gcs)

package storage

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestWithFileOptions(t *testing.T) {
	tests := []struct{ in, want string }{
		{"file:///share/x", "file:///share/x?create_dir=true&no_tmp_dir=true"},
		{"file://./backups", "file://./backups?create_dir=true&no_tmp_dir=true"},
		{"file:///x?metadata=skip", "file:///x?metadata=skip&create_dir=true&no_tmp_dir=true"},
		{"file:///x?create_dir=true", "file:///x?create_dir=true&no_tmp_dir=true"},
		{"s3://bucket", "s3://bucket"},
	}
	for _, tt := range tests {
		if got := withFileOptions(tt.in); got != tt.want {
			t.Errorf("withFileOptions(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestOpenFileCreatesMissingDirectory(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, "file://"+filepath.Join(t.TempDir(), "does", "not", "exist"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if _, err := store.Put(ctx, "p/a.tar", strings.NewReader("data")); err != nil {
		t.Fatal(err)
	}
	if keys, err := store.List(ctx, "p/"); err != nil || len(keys) != 1 {
		t.Errorf("keys, err = %v, %v", keys, err)
	}
}
