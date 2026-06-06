package storage

import (
	"context"
	"fmt"
	"io"
	"strings"

	"gocloud.dev/blob"
	_ "gocloud.dev/blob/fileblob" // file://
	_ "gocloud.dev/blob/gcsblob"  // gs://
	_ "gocloud.dev/blob/s3blob"   // s3://
)

// ObjectStore abstracts backup storage across S3, GCS, and local filesystems.
type ObjectStore interface {
	Put(ctx context.Context, key string, r io.Reader) (int64, error)
	List(ctx context.Context, prefix string) ([]string, error)
	Delete(ctx context.Context, key string) error
	Close() error
}

type blobStore struct {
	b *blob.Bucket
}

// Open opens an object store from a gocloud.dev URL.
//
// Supported schemes:
//   - file://  — local filesystem
//   - s3://    — Amazon S3 or S3-compatible (configure via AWS_ENDPOINT_URL, AWS_ACCESS_KEY_ID, etc.)
//   - gs://    — Google Cloud Storage
func Open(ctx context.Context, storageURL string) (ObjectStore, error) {
	b, err := blob.OpenBucket(ctx, storageURL)
	if err != nil {
		return nil, fmt.Errorf("open bucket %q: %w", storageURL, err)
	}
	return &blobStore{b: b}, nil
}

func (s *blobStore) Put(ctx context.Context, key string, r io.Reader) (int64, error) {
	w, err := s.b.NewWriter(ctx, key, nil)
	if err != nil {
		return 0, fmt.Errorf("create writer: %w", err)
	}
	n, err := io.Copy(w, r)
	if err != nil {
		_ = w.Close()
		return n, fmt.Errorf("write data: %w", err)
	}
	if err := w.Close(); err != nil {
		return n, fmt.Errorf("close writer: %w", err)
	}
	return n, nil
}

func (s *blobStore) List(ctx context.Context, prefix string) ([]string, error) {
	var keys []string
	iter := s.b.List(&blob.ListOptions{Prefix: prefix})
	for {
		obj, err := iter.Next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("iterate objects: %w", err)
		}
		if strings.HasSuffix(obj.Key, ".tar") {
			keys = append(keys, obj.Key)
		}
	}
	return keys, nil
}

func (s *blobStore) Delete(ctx context.Context, key string) error {
	if err := s.b.Delete(ctx, key); err != nil {
		return fmt.Errorf("delete %q: %w", key, err)
	}
	return nil
}

func (s *blobStore) Close() error {
	return s.b.Close()
}
