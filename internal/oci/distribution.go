package oci

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/identity"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/registry"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/storage"
)

type uploadSession struct {
	ID        string
	RepoName  string
	ProjectID string
	CreatedAt time.Time
}

type Distribution struct {
	pool     *pgxpool.Pool
	store    storage.Store
	registry *registry.Service
	mu       sync.Mutex
	uploads  map[string]*uploadSession
}

func NewDistribution(pool *pgxpool.Pool, store storage.Store, reg *registry.Service) *Distribution {
	return &Distribution{
		pool:     pool,
		store:    store,
		registry: reg,
		uploads:  map[string]*uploadSession{},
	}
}

func (d *Distribution) ResolveProjectRepo(ctx context.Context, name string) (projectID, repoID string, err error) {
	parts := strings.SplitN(name, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("%w: repository name must be project/repo", identity.ErrInvalidInput)
	}
	projectSlug := parts[0]
	repoName := parts[1]
	err = d.pool.QueryRow(ctx, `
		SELECT p.id FROM projects p WHERE p.slug = $1 LIMIT 1
	`, projectSlug).Scan(&projectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", identity.ErrNotFound
	}
	if err != nil {
		return "", "", err
	}
	repo, err := d.registry.EnsureRepository(ctx, projectID, repoName)
	if err != nil {
		return "", "", err
	}
	return projectID, repo.ID, nil
}

func (d *Distribution) StartUpload(projectID, repoName string) (string, error) {
	id := make([]byte, 16)
	_, _ = rand.Read(id)
	uuid := hex.EncodeToString(id)
	d.mu.Lock()
	defer d.mu.Unlock()
	d.uploads[uuid] = &uploadSession{ID: uuid, RepoName: repoName, ProjectID: projectID, CreatedAt: time.Now().UTC()}
	return uuid, nil
}

func (d *Distribution) CompleteUpload(ctx context.Context, uploadID string, r io.Reader, size int64) (string, int64, error) {
	d.mu.Lock()
	sess, ok := d.uploads[uploadID]
	if ok {
		delete(d.uploads, uploadID)
	}
	d.mu.Unlock()
	if !ok {
		return "", 0, identity.ErrNotFound
	}
	_ = sess
	return d.registry.PutBlob(ctx, r, size)
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
