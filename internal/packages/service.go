package packages

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/identity"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/storage"
)

type Repository struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Name      string    `json:"name"`
	Format    string    `json:"format"`
	CreatedAt time.Time `json:"created_at"`
}

type Version struct {
	ID           string          `json:"id"`
	RepositoryID string          `json:"repository_id"`
	Name         string          `json:"name"`
	Version      string          `json:"version"`
	Filename     string          `json:"filename"`
	Digest       string          `json:"digest"`
	SizeBytes    int64           `json:"size_bytes"`
	CreatedAt    time.Time       `json:"created_at"`
	Metadata     json.RawMessage `json:"-"`
}

type Service struct {
	pool  *pgxpool.Pool
	store storage.Store
}

func New(pool *pgxpool.Pool, store storage.Store) *Service {
	return &Service{pool: pool, store: store}
}

func (s *Service) CreateRepository(ctx context.Context, projectID, name, format string) (Repository, error) {
	name = strings.TrimSpace(name)
	format = strings.ToLower(strings.TrimSpace(format))
	if name == "" {
		return Repository{}, fmt.Errorf("%w: name required", identity.ErrInvalidInput)
	}
	if format == "" {
		format = "generic"
	}
	if format != "generic" && format != "maven" && format != "npm" {
		return Repository{}, fmt.Errorf("%w: unsupported format", identity.ErrInvalidInput)
	}
	var r Repository
	err := s.pool.QueryRow(ctx, `
		INSERT INTO package_repositories (project_id, name, format)
		VALUES ($1,$2,$3)
		RETURNING id, project_id, name, format, created_at
	`, projectID, name, format).Scan(&r.ID, &r.ProjectID, &r.Name, &r.Format, &r.CreatedAt)
	if err != nil && strings.Contains(err.Error(), "SQLSTATE 23505") {
		return Repository{}, identity.ErrConflict
	}
	return r, err
}

func (s *Service) ListRepositories(ctx context.Context, projectID string) ([]Repository, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, project_id, name, format, created_at
		FROM package_repositories WHERE project_id = $1 ORDER BY name
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Repository
	for rows.Next() {
		var r Repository
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.Name, &r.Format, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Service) Publish(ctx context.Context, repoID, name, version, filename string, r io.Reader, size int64, metadata json.RawMessage) (Version, error) {
	name = strings.TrimSpace(name)
	version = strings.TrimSpace(version)
	if name == "" || version == "" {
		return Version{}, fmt.Errorf("%w: name and version required", identity.ErrInvalidInput)
	}
	info, err := s.store.Put(ctx, "", r, size)
	if err != nil {
		return Version{}, err
	}
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}
	var v Version
	err = s.pool.QueryRow(ctx, `
		INSERT INTO package_versions (repository_id, name, version, filename, digest, size_bytes, metadata)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, repository_id, name, version, filename, digest, size_bytes, created_at, metadata
	`, repoID, name, version, filename, info.Digest, info.Size, metadata).
		Scan(&v.ID, &v.RepositoryID, &v.Name, &v.Version, &v.Filename, &v.Digest, &v.SizeBytes, &v.CreatedAt, &v.Metadata)
	if err != nil && strings.Contains(err.Error(), "SQLSTATE 23505") {
		return Version{}, identity.ErrConflict
	}
	return v, err
}

func (s *Service) PublishOrReplace(ctx context.Context, repoID, name, version, filename string, r io.Reader, size int64) (Version, error) {
	name = strings.TrimSpace(name)
	version = strings.TrimSpace(version)
	if name == "" || version == "" {
		return Version{}, fmt.Errorf("%w: name and version required", identity.ErrInvalidInput)
	}
	info, err := s.store.Put(ctx, "", r, size)
	if err != nil {
		return Version{}, err
	}
	var v Version
	err = s.pool.QueryRow(ctx, `
		INSERT INTO package_versions (repository_id, name, version, filename, digest, size_bytes)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (repository_id, name, version, filename)
		DO UPDATE SET digest = EXCLUDED.digest, size_bytes = EXCLUDED.size_bytes, created_at = now()
		RETURNING id, repository_id, name, version, filename, digest, size_bytes, created_at, metadata
	`, repoID, name, version, filename, info.Digest, info.Size).
		Scan(&v.ID, &v.RepositoryID, &v.Name, &v.Version, &v.Filename, &v.Digest, &v.SizeBytes, &v.CreatedAt, &v.Metadata)
	return v, err
}

func (s *Service) ListVersions(ctx context.Context, repoID string) ([]Version, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, repository_id, name, version, filename, digest, size_bytes, created_at
		FROM package_versions WHERE repository_id = $1
		ORDER BY created_at DESC LIMIT 50000
	`, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Version
	for rows.Next() {
		var v Version
		if err := rows.Scan(&v.ID, &v.RepositoryID, &v.Name, &v.Version, &v.Filename, &v.Digest, &v.SizeBytes, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Service) GetRepository(ctx context.Context, projectID, repoID string) (Repository, error) {
	var r Repository
	err := s.pool.QueryRow(ctx, `
		SELECT id, project_id, name, format, created_at
		FROM package_repositories WHERE id::text = $1 AND project_id = $2
	`, repoID, projectID).Scan(&r.ID, &r.ProjectID, &r.Name, &r.Format, &r.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Repository{}, identity.ErrNotFound
	}
	if err != nil {
		return Repository{}, err
	}
	return r, nil
}

func (s *Service) GetRepositoryByName(ctx context.Context, projectID, name, format string) (Repository, error) {
	var r Repository
	err := s.pool.QueryRow(ctx, `
		SELECT id, project_id, name, format, created_at
		FROM package_repositories
		WHERE project_id = $1 AND name = $2 AND format = $3
	`, projectID, name, format).Scan(&r.ID, &r.ProjectID, &r.Name, &r.Format, &r.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Repository{}, identity.ErrNotFound
	}
	if err != nil {
		return Repository{}, err
	}
	return r, nil
}

func (s *Service) ListVersionsByName(ctx context.Context, repoID, name string) ([]Version, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, repository_id, name, version, filename, digest, size_bytes, created_at, metadata
		FROM package_versions
		WHERE repository_id = $1 AND name = $2
		ORDER BY created_at DESC
	`, repoID, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Version
	for rows.Next() {
		var v Version
		if err := rows.Scan(&v.ID, &v.RepositoryID, &v.Name, &v.Version, &v.Filename, &v.Digest, &v.SizeBytes, &v.CreatedAt, &v.Metadata); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Service) GetVersion(ctx context.Context, repoID, name, version, filename string) (Version, error) {
	var v Version
	err := s.pool.QueryRow(ctx, `
		SELECT id, repository_id, name, version, filename, digest, size_bytes, created_at, metadata
		FROM package_versions
		WHERE repository_id = $1 AND name = $2 AND version = $3 AND filename = $4
	`, repoID, name, version, filename).Scan(&v.ID, &v.RepositoryID, &v.Name, &v.Version, &v.Filename, &v.Digest, &v.SizeBytes, &v.CreatedAt, &v.Metadata)
	if errors.Is(err, pgx.ErrNoRows) {
		return Version{}, identity.ErrNotFound
	}
	if err != nil {
		return Version{}, err
	}
	return v, nil
}

func (s *Service) Open(ctx context.Context, digest string) (io.ReadCloser, storage.BlobInfo, error) {
	return s.store.Get(ctx, digest)
}
