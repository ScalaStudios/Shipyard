package releases

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/identity"
)

type Environment struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type Release struct {
	ID             string    `json:"id"`
	ProjectID      string    `json:"project_id"`
	OrganizationID string    `json:"organization_id"`
	Version        string    `json:"version"`
	Title          string    `json:"title"`
	Notes          string    `json:"notes"`
	RunID          string    `json:"run_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type Deployment struct {
	ID            string     `json:"id"`
	ProjectID     string     `json:"project_id"`
	EnvironmentID string     `json:"environment_id"`
	ReleaseID     string     `json:"release_id"`
	Status        string     `json:"status"`
	CreatedAt     time.Time  `json:"created_at"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
}

type Service struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func (s *Service) CreateEnvironment(ctx context.Context, projectID, slug, name string) (Environment, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	name = strings.TrimSpace(name)
	if slug == "" || name == "" {
		return Environment{}, fmt.Errorf("%w: slug and name required", identity.ErrInvalidInput)
	}
	var e Environment
	err := s.pool.QueryRow(ctx, `
		INSERT INTO environments (project_id, slug, name) VALUES ($1,$2,$3)
		RETURNING id, project_id, slug, name, created_at
	`, projectID, slug, name).Scan(&e.ID, &e.ProjectID, &e.Slug, &e.Name, &e.CreatedAt)
	if err != nil && strings.Contains(err.Error(), "SQLSTATE 23505") {
		return Environment{}, identity.ErrConflict
	}
	return e, err
}

func (s *Service) ListEnvironments(ctx context.Context, projectID string) ([]Environment, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, project_id, slug, name, created_at FROM environments WHERE project_id = $1 ORDER BY slug
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Environment
	for rows.Next() {
		var e Environment
		if err := rows.Scan(&e.ID, &e.ProjectID, &e.Slug, &e.Name, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Service) CreateRelease(ctx context.Context, orgID, projectID, actorID, version, title, notes, runID string, artifactIDs []string) (Release, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return Release{}, fmt.Errorf("%w: version required", identity.ErrInvalidInput)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Release{}, err
	}
	defer tx.Rollback(ctx)

	var rel Release
	err = tx.QueryRow(ctx, `
		INSERT INTO releases (project_id, organization_id, version, title, notes, run_id, created_by)
		VALUES ($1,$2,$3,$4,$5,NULLIF($6,'')::uuid,$7)
		RETURNING id, project_id, organization_id, version, title, notes, COALESCE(run_id::text,''), created_at
	`, projectID, orgID, version, title, notes, runID, actorID).
		Scan(&rel.ID, &rel.ProjectID, &rel.OrganizationID, &rel.Version, &rel.Title, &rel.Notes, &rel.RunID, &rel.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "SQLSTATE 23505") {
			return Release{}, identity.ErrConflict
		}
		return Release{}, err
	}
	for _, artifactID := range artifactIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO release_artifacts (release_id, artifact_id) VALUES ($1,$2)
		`, rel.ID, artifactID); err != nil {
			return Release{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Release{}, err
	}
	return rel, nil
}

func (s *Service) ListReleases(ctx context.Context, projectID string) ([]Release, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, project_id, organization_id, version, title, notes, COALESCE(run_id::text,''), created_at
		FROM releases WHERE project_id = $1 ORDER BY created_at DESC LIMIT 100
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Release
	for rows.Next() {
		var r Release
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.OrganizationID, &r.Version, &r.Title, &r.Notes, &r.RunID, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Service) CreateDeployment(ctx context.Context, projectID, envID, releaseID, actorID string) (Deployment, error) {
	var d Deployment
	err := s.pool.QueryRow(ctx, `
		INSERT INTO deployments (project_id, environment_id, release_id, status, created_by, finished_at)
		VALUES ($1,$2,$3,'succeeded',$4,now())
		RETURNING id, project_id, environment_id, release_id, status, created_at, finished_at
	`, projectID, envID, releaseID, actorID).
		Scan(&d.ID, &d.ProjectID, &d.EnvironmentID, &d.ReleaseID, &d.Status, &d.CreatedAt, &d.FinishedAt)
	return d, err
}

func (s *Service) ListDeployments(ctx context.Context, projectID string) ([]Deployment, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, project_id, environment_id, release_id, status, created_at, finished_at
		FROM deployments WHERE project_id = $1 ORDER BY created_at DESC LIMIT 100
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Deployment
	for rows.Next() {
		var d Deployment
		if err := rows.Scan(&d.ID, &d.ProjectID, &d.EnvironmentID, &d.ReleaseID, &d.Status, &d.CreatedAt, &d.FinishedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Service) GetRelease(ctx context.Context, projectID, releaseID string) (Release, error) {
	var r Release
	err := s.pool.QueryRow(ctx, `
		SELECT id, project_id, organization_id, version, title, notes, COALESCE(run_id::text,''), created_at
		FROM releases WHERE id = $1 AND project_id = $2
	`, releaseID, projectID).Scan(&r.ID, &r.ProjectID, &r.OrganizationID, &r.Version, &r.Title, &r.Notes, &r.RunID, &r.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Release{}, identity.ErrNotFound
	}
	return r, err
}
