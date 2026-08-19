package storage

import (
	"context"
	"io"
)

type BlobInfo struct {
	Digest string
	Size   int64
}

type Store interface {
	Put(ctx context.Context, digest string, r io.Reader, size int64) (BlobInfo, error)
	Get(ctx context.Context, digest string) (io.ReadCloser, BlobInfo, error)
	Stat(ctx context.Context, digest string) (BlobInfo, error)
	Exists(ctx context.Context, digest string) (bool, error)
	Delete(ctx context.Context, digest string) error
}
