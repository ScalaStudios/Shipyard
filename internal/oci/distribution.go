package oci

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/identity"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/registry"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/storage"
)

type uploadSession struct {
	ID        string
	RepoName  string
	ProjectID string
	Path      string
	Size      int64
	CreatedAt time.Time
}

type Distribution struct {
	pool     *pgxpool.Pool
	store    storage.Store
	registry *registry.Service
	dir      string
	mu       sync.Mutex
	uploads  map[string]*uploadSession
}

func NewDistribution(pool *pgxpool.Pool, store storage.Store, reg *registry.Service) *Distribution {
	dir := filepath.Join(os.TempDir(), "shipyard-oci-uploads")
	_ = os.MkdirAll(dir, 0o700)
	return &Distribution{
		pool:     pool,
		store:    store,
		registry: reg,
		dir:      dir,
		uploads:  map[string]*uploadSession{},
	}
}

func (d *Distribution) StartUpload(projectID, repoName string) (string, error) {
	id := make([]byte, 16)
	_, _ = rand.Read(id)
	uuid := hex.EncodeToString(id)
	path := filepath.Join(d.dir, uuid)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	f.Close()
	d.mu.Lock()
	defer d.mu.Unlock()
	d.sweepLocked()
	d.uploads[uuid] = &uploadSession{ID: uuid, RepoName: repoName, ProjectID: projectID, Path: path, CreatedAt: time.Now().UTC()}
	return uuid, nil
}

func (d *Distribution) sweepLocked() {
	cutoff := time.Now().UTC().Add(-time.Hour)
	for id, sess := range d.uploads {
		if sess.CreatedAt.Before(cutoff) {
			os.Remove(sess.Path)
			delete(d.uploads, id)
		}
	}
}

func (d *Distribution) AppendUpload(uploadID string, r io.Reader) (int64, error) {
	d.mu.Lock()
	sess, ok := d.uploads[uploadID]
	d.mu.Unlock()
	if !ok {
		return 0, identity.ErrNotFound
	}
	f, err := os.OpenFile(sess.Path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	n, err := io.Copy(f, r)
	if err != nil {
		return 0, err
	}
	d.mu.Lock()
	sess.Size += n
	size := sess.Size
	d.mu.Unlock()
	return size, nil
}

func (d *Distribution) CompleteUpload(ctx context.Context, uploadID string, r io.Reader, expectedDigest string) (string, int64, error) {
	d.mu.Lock()
	sess, ok := d.uploads[uploadID]
	if ok {
		delete(d.uploads, uploadID)
	}
	d.mu.Unlock()
	if !ok {
		return "", 0, identity.ErrNotFound
	}
	defer os.Remove(sess.Path)
	if r != nil {
		af, err := os.OpenFile(sess.Path, os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return "", 0, err
		}
		if _, err := io.Copy(af, r); err != nil {
			af.Close()
			return "", 0, err
		}
		af.Close()
	}
	f, err := os.Open(sess.Path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	digest, size, err := d.registry.PutBlob(ctx, f, -1)
	if err != nil {
		return "", 0, err
	}
	if expectedDigest != "" && !strings.EqualFold(expectedDigest, digest) {
		return "", 0, fmt.Errorf("%w: digest mismatch", identity.ErrInvalidInput)
	}
	return digest, size, nil
}

func (d *Distribution) PutBlobMonolithic(ctx context.Context, r io.Reader, size int64) (string, int64, error) {
	return d.registry.PutBlob(ctx, r, size)
}

func (d *Distribution) GetBlob(ctx context.Context, digest string) (io.ReadCloser, storage.BlobInfo, error) {
	return d.store.Get(ctx, digest)
}

func (d *Distribution) BlobExists(ctx context.Context, digest string) (bool, error) {
	return d.store.Exists(ctx, digest)
}

func (d *Distribution) StatBlob(ctx context.Context, digest string) (storage.BlobInfo, error) {
	return d.store.Stat(ctx, digest)
}
