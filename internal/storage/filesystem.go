package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type FilesystemStore struct {
	root string
}

func NewFilesystemStore(root string) (*FilesystemStore, error) {
	if root == "" {
		return nil, fmt.Errorf("filesystem storage root is required")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create storage root: %w", err)
	}
	probe, err := os.CreateTemp(root, "writable-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("storage root %s is not writable: %w", root, err)
	}
	probe.Close()
	os.Remove(probe.Name())
	return &FilesystemStore{root: root}, nil
}

func (s *FilesystemStore) Put(ctx context.Context, digest string, r io.Reader, size int64) (BlobInfo, error) {
	if err := ctx.Err(); err != nil {
		return BlobInfo{}, err
	}

	tmp, err := os.CreateTemp(s.root, "upload-*.tmp")
	if err != nil {
		return BlobInfo{}, fmt.Errorf("create temp blob: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		tmp.Close()
		os.Remove(tmpName)
	}()

	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(tmp, hasher), r)
	if err != nil {
		return BlobInfo{}, fmt.Errorf("write blob: %w", err)
	}
	if size >= 0 && written != size {
		return BlobInfo{}, fmt.Errorf("blob size mismatch: expected %d got %d", size, written)
	}

	computed := "sha256:" + hex.EncodeToString(hasher.Sum(nil))
	if digest != "" && digest != computed {
		return BlobInfo{}, fmt.Errorf("digest mismatch: expected %s got %s", digest, computed)
	}

	if err := tmp.Close(); err != nil {
		return BlobInfo{}, fmt.Errorf("close temp blob: %w", err)
	}

	dest, err := s.pathFor(computed)
	if err != nil {
		return BlobInfo{}, err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return BlobInfo{}, fmt.Errorf("create blob dir: %w", err)
	}
	if _, err := os.Stat(dest); err == nil {
		return BlobInfo{Digest: computed, Size: written}, nil
	}
	if err := os.Rename(tmpName, dest); err != nil {
		return BlobInfo{}, fmt.Errorf("finalize blob: %w", err)
	}
	return BlobInfo{Digest: computed, Size: written}, nil
}

func (s *FilesystemStore) Get(ctx context.Context, digest string) (io.ReadCloser, BlobInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, BlobInfo{}, err
	}
	path, err := s.pathFor(digest)
	if err != nil {
		return nil, BlobInfo{}, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, BlobInfo{}, fmt.Errorf("open blob: %w", err)
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, BlobInfo{}, fmt.Errorf("stat blob: %w", err)
	}
	return f, BlobInfo{Digest: digest, Size: st.Size()}, nil
}

func (s *FilesystemStore) Exists(ctx context.Context, digest string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	path, err := s.pathFor(digest)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (s *FilesystemStore) Delete(ctx context.Context, digest string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path, err := s.pathFor(digest)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete blob: %w", err)
	}
	return nil
}

func (s *FilesystemStore) pathFor(digest string) (string, error) {
	hexPart, err := normalizeDigest(digest)
	if err != nil {
		return "", err
	}
	if len(hexPart) < 4 {
		return "", fmt.Errorf("digest too short")
	}
	return filepath.Join(s.root, "sha256", hexPart[:2], hexPart[2:4], hexPart), nil
}

func normalizeDigest(digest string) (string, error) {
	digest = strings.TrimSpace(digest)
	if digest == "" {
		return "", fmt.Errorf("digest is required")
	}
	hexPart := digest
	if strings.HasPrefix(digest, "sha256:") {
		hexPart = strings.TrimPrefix(digest, "sha256:")
	}
	if len(hexPart) != 64 {
		return "", fmt.Errorf("invalid sha256 digest")
	}
	for _, c := range hexPart {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return "", fmt.Errorf("invalid sha256 digest")
		}
	}
	return hexPart, nil
}
