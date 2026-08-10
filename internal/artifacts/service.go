package artifacts

import (
	"context"
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

type Artifact struct {
	ID             string    `json:"id"`
	ProjectID      string    `json:"project_id"`
	OrganizationID string    `json:"organization_id"`
	JobID          string    `json:"job_id,omitempty"`
	RunID          string    `json:"run_id,omitempty"`
	Name           string    `json:"name"`
	Digest         string    `json:"digest"`
	SizeBytes      int64     `json:"size_bytes"`
	ContentType    string    `json:"content_type"`
	CreatedAt      time.Time `json:"created_at"`
}

type Service struct {
	pool  *pgxpool.Pool
	store storage.Store
}

func New(pool *pgxpool.Pool, store storage.Store) *Service {
	return &Service{pool: pool, store: store}
}

func (s *Service) Put(ctx context.Context, orgID, projectID, actorID, name, contentType, jobID, runID string, r io.Reader, size int64) (Artifact, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Artifact{}, fmt.Errorf("%w: name required", identity.ErrInvalidInput)
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	info, err := s.store.Put(ctx, "", r, size)
	if err != nil {
		return Artifact{}, err
	}
	var a Artifact
	err = s.pool.QueryRow(ctx, `
		INSERT INTO artifacts (
			project_id, organization_id, job_id, run_id, name, digest, size_bytes, content_type, created_by
		) VALUES ($1,$2,NULLIF($3,'')::uuid,NULLIF($4,'')::uuid,$5,$6,$7,$8,$9)
		RETURNING id, project_id, organization_id, COALESCE(job_id::text,''), COALESCE(run_id::text,''),
		          name, digest, size_bytes, content_type, created_at
	`, projectID, orgID, jobID, runID, name, info.Digest, info.Size, contentType, actorID).
		Scan(&a.ID, &a.ProjectID, &a.OrganizationID, &a.JobID, &a.RunID, &a.Name, &a.Digest, &a.SizeBytes, &a.ContentType, &a.CreatedAt)
	return a, err
}

func (s *Service) List(ctx context.Context, projectID string) ([]Artifact, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, project_id, organization_id, COALESCE(job_id::text,''), COALESCE(run_id::text,''),
		       name, digest, size_bytes, content_type, created_at
		FROM artifacts WHERE project_id = $1
		ORDER BY created_at DESC LIMIT 200
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Artifact
	for rows.Next() {
		var a Artifact
		if err := rows.Scan(&a.ID, &a.ProjectID, &a.OrganizationID, &a.JobID, &a.RunID, &a.Name, &a.Digest, &a.SizeBytes, &a.ContentType, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Service) Get(ctx context.Context, projectID, artifactID string) (Artifact, error) {
	var a Artifact
	err := s.pool.QueryRow(ctx, `
		SELECT id, project_id, organization_id, COALESCE(job_id::text,''), COALESCE(run_id::text,''),
		       name, digest, size_bytes, content_type, created_at
		FROM artifacts WHERE id = $1 AND project_id = $2
	`, artifactID, projectID).Scan(&a.ID, &a.ProjectID, &a.OrganizationID, &a.JobID, &a.RunID, &a.Name, &a.Digest, &a.SizeBytes, &a.ContentType, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Artifact{}, identity.ErrNotFound
	}
	return a, err
}

func (s *Service) Open(ctx context.Context, projectID, artifactID string) (io.ReadCloser, Artifact, error) {
	a, err := s.Get(ctx, projectID, artifactID)
	if err != nil {
		return nil, Artifact{}, err
	}
	rc, _, err := s.store.Get(ctx, a.Digest)
	return rc, a, err
}
