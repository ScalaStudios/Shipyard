package storage

import (
	"context"
	"fmt"
	"io"
)

type S3Config struct {
	Endpoint       string
	Region         string
	Bucket         string
	AccessKey      string
	SecretKey      string
	ForcePathStyle bool
}

type S3Store struct {
	cfg S3Config
}

func NewS3Store(cfg S3Config) (*S3Store, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("s3 bucket is required")
	}
	return &S3Store{cfg: cfg}, nil
}

func (s *S3Store) Put(ctx context.Context, digest string, r io.Reader, size int64) (BlobInfo, error) {
	return BlobInfo{}, fmt.Errorf("s3 storage backend is configured but not yet implemented in phase 0; interface is reserved")
}

func (s *S3Store) Get(ctx context.Context, digest string) (io.ReadCloser, BlobInfo, error) {
	return nil, BlobInfo{}, fmt.Errorf("s3 storage backend is configured but not yet implemented in phase 0; interface is reserved")
}

func (s *S3Store) Exists(ctx context.Context, digest string) (bool, error) {
	return false, fmt.Errorf("s3 storage backend is configured but not yet implemented in phase 0; interface is reserved")
}

func (s *S3Store) Delete(ctx context.Context, digest string) error {
	return fmt.Errorf("s3 storage backend is configured but not yet implemented in phase 0; interface is reserved")
}
