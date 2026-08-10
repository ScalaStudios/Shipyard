package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/identity"
)

type Definition struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	YAML      string    `json:"yaml_source"`
	CreatedAt time.Time `json:"created_at"`
}

type Run struct {
	ID             string     `json:"id"`
	PipelineID     string     `json:"pipeline_id"`
	ProjectID      string     `json:"project_id"`
	OrganizationID string     `json:"organization_id"`
	Number         int64      `json:"number"`
	Status         string     `json:"status"`
	TriggerType    string     `json:"trigger_type"`
	GitRef         string     `json:"git_ref"`
	GitSHA         string     `json:"git_sha"`
	ErrorMessage   string     `json:"error_message,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
}

type Job struct {
	ID            string     `json:"id"`
	RunID         string     `json:"run_id"`
	Name          string     `json:"name"`
	Status        string     `json:"status"`
	Needs         []string   `json:"needs"`
	RunnerLabels  []string   `json:"runner_labels"`
	ErrorMessage  string     `json:"error_message,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
}

type Step struct {
	ID       string `json:"id"`
	JobID    string `json:"job_id"`
	Position int    `json:"position"`
	Name     string `json:"name"`
	Uses     string `json:"uses"`
	Run      string `json:"run_script"`
	Status   string `json:"status"`
}

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func (s *Service) UpsertDefinition(ctx context.Context, projectID, actorID, slug, yamlSource string) (Definition, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == "" {
		return Definition{}, fmt.Errorf("%w: slug required", identity.ErrInvalidInput)
	}
	doc, err := Parse(yamlSource)
	if err != nil {
		return Definition{}, fmt.Errorf("%w: %v", identity.ErrInvalidInput, err)
	}
	name := strings.TrimSpace(doc.Pipeline.Name)
	if name == "" {
		name = slug
	}
	raw, _ := json.Marshal(doc)

	var d Definition
	err = s.pool.QueryRow(ctx, `
		INSERT INTO pipeline_definitions (project_id, name, slug, yaml_source, parsed, created_by)
		VALUES ($1,$2,$3,$4,$5::jsonb,$6)
		ON CONFLICT (project_id, slug) DO UPDATE
		SET name = EXCLUDED.name,
		    yaml_source = EXCLUDED.yaml_source,
		    parsed = EXCLUDED.parsed,
		    updated_at = now()
		RETURNING id, project_id, name, slug, yaml_source, created_at
	`, projectID, name, slug, yamlSource, string(raw), actorID).
		Scan(&d.ID, &d.ProjectID, &d.Name, &d.Slug, &d.YAML, &d.CreatedAt)
	return d, err
}

func (s *Service) ListDefinitions(ctx context.Context, projectID string) ([]Definition, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, project_id, name, slug, yaml_source, created_at
		FROM pipeline_definitions WHERE project_id = $1 ORDER BY slug
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Definition
	for rows.Next() {
		var d Definition
		if err := rows.Scan(&d.ID, &d.ProjectID, &d.Name, &d.Slug, &d.YAML, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Service) GetDefinition(ctx context.Context, projectID, pipelineID string) (Definition, error) {
	var d Definition
	err := s.pool.QueryRow(ctx, `
		SELECT id, project_id, name, slug, yaml_source, created_at
		FROM pipeline_definitions WHERE id = $1 AND project_id = $2
	`, pipelineID, projectID).Scan(&d.ID, &d.ProjectID, &d.Name, &d.Slug, &d.YAML, &d.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Definition{}, identity.ErrNotFound
	}
	return d, err
}

func (s *Service) StartRun(ctx context.Context, orgID, projectID, pipelineID, actorID, triggerType, gitRef, gitSHA string) (Run, error) {
	def, err := s.GetDefinition(ctx, projectID, pipelineID)
	if err != nil {
		return Run{}, err
	}
	doc, err := Parse(def.YAML)
	if err != nil {
		return Run{}, err
	}
	if triggerType == "" {
		triggerType = "manual"
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Run{}, err
	}
	defer tx.Rollback(ctx)

	var number int64
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(number), 0) + 1 FROM pipeline_runs WHERE pipeline_id = $1
	`, pipelineID).Scan(&number); err != nil {
		return Run{}, err
	}

	var run Run
	err = tx.QueryRow(ctx, `
		INSERT INTO pipeline_runs (
			pipeline_id, project_id, organization_id, number, status, trigger_type,
			triggered_by, git_ref, git_sha, queued_at
		) VALUES ($1,$2,$3,$4,'queued',$5,$6,$7,$8,now())
		RETURNING id, pipeline_id, project_id, organization_id, number, status, trigger_type, git_ref, git_sha, created_at
	`, pipelineID, projectID, orgID, number, triggerType, actorID, gitRef, gitSHA).
		Scan(&run.ID, &run.PipelineID, &run.ProjectID, &run.OrganizationID, &run.Number, &run.Status, &run.TriggerType, &run.GitRef, &run.GitSHA, &run.CreatedAt)
	if err != nil {
		return Run{}, err
	}

	for name, job := range doc.Jobs {
		status := "queued"
		if len(job.Needs) > 0 {
			status = "pending"
		}
		var jobID string
		err = tx.QueryRow(ctx, `
			INSERT INTO jobs (run_id, name, status, needs, runner_labels, queued_at)
			VALUES ($1,$2,$3,$4,$5, CASE WHEN $3 = 'queued' THEN now() ELSE NULL END)
			RETURNING id
		`, run.ID, name, status, job.Needs, job.Labels()).Scan(&jobID)
		if err != nil {
			return Run{}, err
		}
		for i, step := range job.Steps {
			stepName := step.Name
			if stepName == "" {
				if step.Run != "" {
					stepName = fmt.Sprintf("run-%d", i+1)
				} else {
					stepName = step.Uses
				}
			}
			withRaw, _ := json.Marshal(step.With)
			if _, err := tx.Exec(ctx, `
				INSERT INTO job_steps (job_id, position, name, uses, run_script, with_args)
				VALUES ($1,$2,$3,$4,$5,$6::jsonb)
			`, jobID, i, stepName, step.Uses, step.Run, string(withRaw)); err != nil {
				return Run{}, err
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return Run{}, err
	}
	return run, nil
}

func (s *Service) ListRuns(ctx context.Context, projectID string) ([]Run, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, pipeline_id, project_id, organization_id, number, status, trigger_type,
		       git_ref, git_sha, error_message, created_at, started_at, finished_at
		FROM pipeline_runs WHERE project_id = $1
		ORDER BY created_at DESC
		LIMIT 100
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Run
	for rows.Next() {
		var r Run
		if err := rows.Scan(&r.ID, &r.PipelineID, &r.ProjectID, &r.OrganizationID, &r.Number, &r.Status, &r.TriggerType,
			&r.GitRef, &r.GitSHA, &r.ErrorMessage, &r.CreatedAt, &r.StartedAt, &r.FinishedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Service) GetRun(ctx context.Context, projectID, runID string) (Run, error) {
	var r Run
	err := s.pool.QueryRow(ctx, `
		SELECT id, pipeline_id, project_id, organization_id, number, status, trigger_type,
		       git_ref, git_sha, error_message, created_at, started_at, finished_at
		FROM pipeline_runs WHERE id = $1 AND project_id = $2
	`, runID, projectID).Scan(&r.ID, &r.PipelineID, &r.ProjectID, &r.OrganizationID, &r.Number, &r.Status, &r.TriggerType,
		&r.GitRef, &r.GitSHA, &r.ErrorMessage, &r.CreatedAt, &r.StartedAt, &r.FinishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Run{}, identity.ErrNotFound
	}
	return r, err
}

func (s *Service) ListJobs(ctx context.Context, runID string) ([]Job, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, run_id, name, status, needs, runner_labels, error_message, created_at, started_at, finished_at
		FROM jobs WHERE run_id = $1 ORDER BY name
	`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Job
	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.ID, &j.RunID, &j.Name, &j.Status, &j.Needs, &j.RunnerLabels, &j.ErrorMessage, &j.CreatedAt, &j.StartedAt, &j.FinishedAt); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func (s *Service) ListSteps(ctx context.Context, jobID string) ([]Step, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, job_id, position, name, uses, run_script, status
		FROM job_steps WHERE job_id = $1 ORDER BY position
	`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Step
	for rows.Next() {
		var st Step
		if err := rows.Scan(&st.ID, &st.JobID, &st.Position, &st.Name, &st.Uses, &st.Run, &st.Status); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

func (s *Service) CancelRun(ctx context.Context, projectID, runID string) error {
	ct, err := s.pool.Exec(ctx, `
		UPDATE pipeline_runs
		SET status = 'canceled', finished_at = now()
		WHERE id = $1 AND project_id = $2 AND status IN ('pending','queued','running')
	`, runID, projectID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return identity.ErrNotFound
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE jobs
		SET status = 'canceled', finished_at = now()
		WHERE run_id = $1 AND status IN ('pending','queued','leased','running')
	`, runID)
	return err
}

func (s *Service) AdvanceRunGraph(ctx context.Context, runID string) error {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, status, needs FROM jobs WHERE run_id = $1
	`, runID)
	if err != nil {
		return err
	}
	defer rows.Close()

	type jobRow struct {
		id, name, status string
		needs            []string
	}
	byName := map[string]jobRow{}
	var list []jobRow
	for rows.Next() {
		var j jobRow
		if err := rows.Scan(&j.id, &j.name, &j.status, &j.needs); err != nil {
			return err
		}
		byName[j.name] = j
		list = append(list, j)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, j := range list {
		if j.status != "pending" {
			continue
		}
		ready := true
		failedDep := false
		for _, need := range j.needs {
			dep := byName[need]
			switch dep.status {
			case "succeeded":
			case "failed", "canceled":
				failedDep = true
				ready = false
			default:
				ready = false
			}
		}
		if failedDep {
			if _, err := s.pool.Exec(ctx, `
				UPDATE jobs SET status = 'skipped', finished_at = now(), error_message = 'dependency failed'
				WHERE id = $1 AND status = 'pending'
			`, j.id); err != nil {
				return err
			}
			continue
		}
		if ready {
			if _, err := s.pool.Exec(ctx, `
				UPDATE jobs SET status = 'queued', queued_at = now()
				WHERE id = $1 AND status = 'pending'
			`, j.id); err != nil {
				return err
			}
		}
	}

	var active, failed, canceled int
	if err := s.pool.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE status IN ('pending','queued','leased','running')),
			count(*) FILTER (WHERE status = 'failed'),
			count(*) FILTER (WHERE status = 'canceled')
		FROM jobs WHERE run_id = $1
	`, runID).Scan(&active, &failed, &canceled); err != nil {
		return err
	}

	switch {
	case active > 0:
		_, err = s.pool.Exec(ctx, `
			UPDATE pipeline_runs
			SET status = 'running', started_at = COALESCE(started_at, now())
			WHERE id = $1 AND status IN ('queued','running')
		`, runID)
	case failed > 0:
		_, err = s.pool.Exec(ctx, `
			UPDATE pipeline_runs SET status = 'failed', finished_at = now()
			WHERE id = $1 AND status IN ('queued','running')
		`, runID)
	case canceled > 0:
		_, err = s.pool.Exec(ctx, `
			UPDATE pipeline_runs SET status = 'canceled', finished_at = now()
			WHERE id = $1 AND status IN ('queued','running')
		`, runID)
	default:
		_, err = s.pool.Exec(ctx, `
			UPDATE pipeline_runs SET status = 'succeeded', finished_at = now()
			WHERE id = $1 AND status IN ('queued','running')
		`, runID)
	}
	return err
}
