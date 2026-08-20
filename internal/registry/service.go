package registry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	CreatedAt time.Time `json:"created_at"`
}

type Manifest struct {
	ID        string    `json:"id"`
	Digest    string    `json:"digest"`
	MediaType string    `json:"media_type"`
	SizeBytes int64     `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at"`
}

type Service struct {
	pool  *pgxpool.Pool
	store storage.Store
}

func New(pool *pgxpool.Pool, store storage.Store) *Service {
	return &Service{pool: pool, store: store}
}

func (s *Service) EnsureRepository(ctx context.Context, projectID, name string) (Repository, error) {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return Repository{}, fmt.Errorf("%w: name required", identity.ErrInvalidInput)
	}
	var r Repository
	err := s.pool.QueryRow(ctx, `
		INSERT INTO oci_repositories (project_id, name) VALUES ($1,$2)
		ON CONFLICT (project_id, name) DO UPDATE SET name = EXCLUDED.name
		RETURNING id, project_id, name, created_at
	`, projectID, name).Scan(&r.ID, &r.ProjectID, &r.Name, &r.CreatedAt)
	return r, err
}

func (s *Service) ListRepositories(ctx context.Context, projectID string) ([]Repository, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, project_id, name, created_at FROM oci_repositories WHERE project_id = $1 ORDER BY name
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Repository
	for rows.Next() {
		var r Repository
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.Name, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Service) PutBlob(ctx context.Context, r io.Reader, size int64) (string, int64, error) {
	info, err := s.store.Put(ctx, "", r, size)
	if err != nil {
		return "", 0, err
	}
	return info.Digest, info.Size, nil
}

func (s *Service) PutManifest(ctx context.Context, repoID, mediaType, tag string, raw []byte) (Manifest, error) {
	if mediaType == "" {
		mediaType = "application/vnd.oci.image.manifest.v1+json"
	}
	sum := sha256.Sum256(raw)
	digest := "sha256:" + hex.EncodeToString(sum[:])
	if !json.Valid(raw) {
		return Manifest{}, fmt.Errorf("%w: manifest must be json", identity.ErrInvalidInput)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Manifest{}, err
	}
	defer tx.Rollback(ctx)

	var m Manifest
	err = tx.QueryRow(ctx, `
		INSERT INTO oci_manifests (repository_id, digest, media_type, raw, size_bytes)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (repository_id, digest) DO UPDATE SET media_type = EXCLUDED.media_type
		RETURNING id, digest, media_type, size_bytes, created_at
	`, repoID, digest, mediaType, raw, len(raw)).
		Scan(&m.ID, &m.Digest, &m.MediaType, &m.SizeBytes, &m.CreatedAt)
	if err != nil {
		return Manifest{}, err
	}

	if tag != "" {
		if _, err := tx.Exec(ctx, `
			INSERT INTO oci_tags (repository_id, name, manifest_id)
			VALUES ($1,$2,$3)
			ON CONFLICT (repository_id, name) DO UPDATE
			SET manifest_id = EXCLUDED.manifest_id, updated_at = now()
		`, repoID, tag, m.ID); err != nil {
			return Manifest{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func (s *Service) GetRepository(ctx context.Context, projectID, name string) (Repository, error) {
	name = strings.TrimSpace(strings.ToLower(name))
	var r Repository
	err := s.pool.QueryRow(ctx, `
		SELECT id, project_id, name, created_at FROM oci_repositories WHERE project_id = $1 AND name = $2
	`, projectID, name).Scan(&r.ID, &r.ProjectID, &r.Name, &r.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Repository{}, identity.ErrNotFound
	}
	return r, err
}

func (s *Service) GetManifest(ctx context.Context, repoID, ref string) (Manifest, []byte, error) {
	var m Manifest
	var raw []byte
	var err error
	if strings.HasPrefix(ref, "sha256:") {
		err = s.pool.QueryRow(ctx, `
			SELECT id, digest, media_type, size_bytes, created_at, raw
			FROM oci_manifests
			WHERE repository_id = $1 AND digest = $2
		`, repoID, ref).Scan(&m.ID, &m.Digest, &m.MediaType, &m.SizeBytes, &m.CreatedAt, &raw)
	} else {
		err = s.pool.QueryRow(ctx, `
			SELECT m.id, m.digest, m.media_type, m.size_bytes, m.created_at, m.raw
			FROM oci_tags t
			JOIN oci_manifests m ON m.id = t.manifest_id
			WHERE t.repository_id = $1 AND t.name = $2
		`, repoID, ref).Scan(&m.ID, &m.Digest, &m.MediaType, &m.SizeBytes, &m.CreatedAt, &raw)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return Manifest{}, nil, identity.ErrNotFound
	}
	return m, raw, err
}

func (s *Service) ListTags(ctx context.Context, repoID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT name FROM oci_tags WHERE repository_id = $1 ORDER BY name`, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}
