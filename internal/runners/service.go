package runners

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/auth"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/identity"
)

type Runner struct {
	ID              string     `json:"id"`
	OrganizationID  string     `json:"organization_id,omitempty"`
	Name            string     `json:"name"`
	Labels          []string   `json:"labels"`
	Capabilities    []string   `json:"capabilities"`
	Status          string     `json:"status"`
	LastHeartbeatAt *time.Time `json:"last_heartbeat_at,omitempty"`
	Drained         bool       `json:"drained"`
	CreatedAt       time.Time  `json:"created_at"`
}

type LeaseJob struct {
	ID           string            `json:"id"`
	RunID        string            `json:"run_id"`
	Name         string            `json:"name"`
	RunnerLabels []string          `json:"runner_labels"`
	LeaseID      string            `json:"lease_id"`
	Secrets      map[string]string `json:"secrets,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
	Run          *LeaseRun         `json:"run,omitempty"`
	Repo         *LeaseRepo        `json:"repo,omitempty"`
}

type LeaseRun struct {
	ID          string `json:"id"`
	Number      int64  `json:"number"`
	GitRef      string `json:"git_ref"`
	GitSHA      string `json:"git_sha"`
	ProjectSlug string `json:"project_slug"`
	OrgSlug     string `json:"org_slug"`
}

type LeaseRepo struct {
	CloneURL string `json:"clone_url"`
	Username string `json:"username"`
	Token    string `json:"token"`
}

type Step struct {
	ID       string `json:"id"`
	Position int    `json:"position"`
	Name     string `json:"name"`
	Uses     string `json:"uses"`
	Run      string `json:"run_script"`
}

type Service struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func (s *Service) CreateRegistrationToken(ctx context.Context, orgID, actorID string, ttl time.Duration) (plain string, expires time.Time, err error) {
	plain, err = auth.NewToken(24)
	if err != nil {
		return "", time.Time{}, err
	}
	expires = time.Now().UTC().Add(ttl)
	_, err = s.pool.Exec(ctx, `
		INSERT INTO runner_registration_tokens (token_hash, token_prefix, organization_id, created_by, expires_at)
		VALUES ($1,$2,$3,$4,$5)
	`, auth.HashToken(plain), auth.TokenPrefix(plain), nullIfEmpty(orgID), actorID, expires)
	return plain, expires, err
}

// PeekRegistrationToken reports whether a registration token is still valid (does not consume it).
func (s *Service) PeekRegistrationToken(ctx context.Context, plain string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM runner_registration_tokens
			WHERE token_hash = $1 AND consumed_at IS NULL AND expires_at > now()
		)
	`, auth.HashToken(plain)).Scan(&ok)
	return ok, err
}

func (s *Service) Register(ctx context.Context, registrationToken, name string, labels, capabilities []string) (Runner, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Runner{}, "", fmt.Errorf("%w: name required", identity.ErrInvalidInput)
	}
	labels = lowerLabels(labels)
	if len(labels) == 0 {
		labels = []string{"linux"}
	}
	if capabilities == nil {
		capabilities = []string{}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Runner{}, "", err
	}
	defer tx.Rollback(ctx)

	var orgID *string
	var tokenID string
	err = tx.QueryRow(ctx, `
		SELECT id, organization_id FROM runner_registration_tokens
		WHERE token_hash = $1 AND consumed_at IS NULL AND expires_at > now()
	`, auth.HashToken(registrationToken)).Scan(&tokenID, &orgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Runner{}, "", identity.ErrUnauthorized
	}
	if err != nil {
		return Runner{}, "", err
	}

	runnerToken, err := auth.NewToken(32)
	if err != nil {
		return Runner{}, "", err
	}

	var r Runner
	err = tx.QueryRow(ctx, `
		UPDATE runners
		SET token_hash = $2, token_prefix = $3, labels = $4, capabilities = $5,
			status = 'idle', drained = FALSE, last_heartbeat_at = now()
		WHERE id = (
			SELECT id FROM runners
			WHERE name = $1 AND organization_id IS NOT DISTINCT FROM $6
			ORDER BY created_at DESC
			LIMIT 1
			FOR UPDATE
		)
		RETURNING id, COALESCE(organization_id::text, ''), name, labels, capabilities, status, last_heartbeat_at, drained, created_at
	`, name, auth.HashToken(runnerToken), auth.TokenPrefix(runnerToken), labels, capabilities, orgID).
		Scan(&r.ID, &r.OrganizationID, &r.Name, &r.Labels, &r.Capabilities, &r.Status, &r.LastHeartbeatAt, &r.Drained, &r.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `
			INSERT INTO runners (name, token_hash, token_prefix, labels, capabilities, status, organization_id, last_heartbeat_at)
			VALUES ($1,$2,$3,$4,$5,'idle',$6,now())
			RETURNING id, COALESCE(organization_id::text, ''), name, labels, capabilities, status, last_heartbeat_at, drained, created_at
		`, name, auth.HashToken(runnerToken), auth.TokenPrefix(runnerToken), labels, capabilities, orgID).
			Scan(&r.ID, &r.OrganizationID, &r.Name, &r.Labels, &r.Capabilities, &r.Status, &r.LastHeartbeatAt, &r.Drained, &r.CreatedAt)
	}
	if err != nil {
		return Runner{}, "", err
	}

	if _, err := tx.Exec(ctx, `UPDATE runner_registration_tokens SET consumed_at = now() WHERE id = $1`, tokenID); err != nil {
		return Runner{}, "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return Runner{}, "", err
	}
	return r, runnerToken, nil
}

func (s *Service) RunnerFromToken(ctx context.Context, token string) (Runner, error) {
	var r Runner
	err := s.pool.QueryRow(ctx, `
		SELECT id, COALESCE(organization_id::text, ''), name, labels, capabilities, status, last_heartbeat_at, drained, created_at
		FROM runners WHERE token_hash = $1
	`, auth.HashToken(token)).Scan(&r.ID, &r.OrganizationID, &r.Name, &r.Labels, &r.Capabilities, &r.Status, &r.LastHeartbeatAt, &r.Drained, &r.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Runner{}, identity.ErrUnauthorized
	}
	return r, err
}

func (s *Service) Heartbeat(ctx context.Context, runnerID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE runners
		SET last_heartbeat_at = now(),
		    status = CASE WHEN drained THEN 'draining' WHEN status = 'busy' THEN 'busy' ELSE 'idle' END
		WHERE id = $1
	`, runnerID)
	return err
}

func (s *Service) Delete(ctx context.Context, id string) error {
	var status string
	err := s.pool.QueryRow(ctx, `SELECT status FROM runners WHERE id = $1`, id).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.ErrNotFound
	}
	if err != nil {
		return err
	}
	if status == "busy" {
		return fmt.Errorf("%w: runner is running a job; drain it first", identity.ErrInvalidInput)
	}
	if _, err := s.pool.Exec(ctx, `UPDATE jobs SET runner_id = NULL WHERE runner_id = $1`, id); err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM runners WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return identity.ErrNotFound
	}
	return nil
}

func (s *Service) List(ctx context.Context) ([]Runner, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, COALESCE(organization_id::text, ''), name, labels, capabilities, status, last_heartbeat_at, drained, created_at
		FROM runners ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRunners(rows)
}

func (s *Service) ListForOrgs(ctx context.Context, orgIDs []string) ([]Runner, error) {
	if orgIDs == nil {
		orgIDs = []string{}
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, COALESCE(organization_id::text, ''), name, labels, capabilities, status, last_heartbeat_at, drained, created_at
		FROM runners
		WHERE organization_id IS NULL OR organization_id::text = ANY($1)
		ORDER BY name
	`, orgIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRunners(rows)
}

func scanRunners(rows pgx.Rows) ([]Runner, error) {
	var out []Runner
	for rows.Next() {
		var r Runner
		if err := rows.Scan(&r.ID, &r.OrganizationID, &r.Name, &r.Labels, &r.Capabilities, &r.Status, &r.LastHeartbeatAt, &r.Drained, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Service) LeaseNextJob(ctx context.Context, runner Runner, leaseTTL time.Duration) (*LeaseJob, error) {
	if runner.Drained {
		return nil, nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var job LeaseJob
	err = tx.QueryRow(ctx, `
		SELECT j.id, j.run_id, j.name, j.runner_labels
		FROM jobs j
		JOIN pipeline_runs r ON r.id = j.run_id
		WHERE j.status = 'queued'
		  AND ($1 = '' OR r.organization_id::text = $1)
		  AND j.runner_labels <@ $2::text[]
		ORDER BY j.queued_at NULLS FIRST, j.created_at
		FOR UPDATE OF j SKIP LOCKED
		LIMIT 1
	`, runner.OrganizationID, lowerLabels(runner.Labels)).Scan(&job.ID, &job.RunID, &job.Name, &job.RunnerLabels)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, tx.Commit(ctx)
	}
	if err != nil {
		return nil, err
	}
	selected := &job

	leaseID, err := auth.NewToken(16)
	if err != nil {
		return nil, err
	}
	expires := time.Now().UTC().Add(leaseTTL)
	ct, err := tx.Exec(ctx, `
		UPDATE jobs
		SET status = 'leased', runner_id = $2, lease_id = $3, lease_expires_at = $4, started_at = COALESCE(started_at, now())
		WHERE id = $1 AND status = 'queued'
	`, selected.ID, runner.ID, leaseID, expires)
	if err != nil {
		return nil, err
	}
	if ct.RowsAffected() == 0 {
		return nil, tx.Commit(ctx)
	}
	if _, err := tx.Exec(ctx, `UPDATE runners SET status = 'busy' WHERE id = $1`, runner.ID); err != nil {
		return nil, err
	}
	selected.LeaseID = leaseID
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return selected, nil
}

func (s *Service) requireJobRunner(ctx context.Context, jobID, runnerID string) error {
	var ok int
	err := s.pool.QueryRow(ctx, `
		SELECT 1 FROM jobs WHERE id::text = $1 AND runner_id = $2 AND status IN ('leased','running')
	`, jobID, runnerID).Scan(&ok)
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.ErrForbidden
	}
	return err
}

func (s *Service) GetJobSteps(ctx context.Context, runnerID, jobID string) ([]Step, error) {
	if err := s.requireJobRunner(ctx, jobID, runnerID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, position, name, uses, run_script
		FROM job_steps WHERE job_id = $1 ORDER BY position
	`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Step
	for rows.Next() {
		var st Step
		if err := rows.Scan(&st.ID, &st.Position, &st.Name, &st.Uses, &st.Run); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

func (s *Service) MarkJobRunning(ctx context.Context, jobID, leaseID string) error {
	ct, err := s.pool.Exec(ctx, `
		UPDATE jobs SET status = 'running', started_at = COALESCE(started_at, now()),
		    lease_expires_at = now() + interval '2 minutes'
		WHERE id = $1 AND lease_id = $2 AND status = 'leased'
	`, jobID, leaseID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return identity.ErrForbidden
	}
	return nil
}

func (s *Service) RenewLease(ctx context.Context, runnerID, jobID, leaseID string, ttl time.Duration) (string, error) {
	var status string
	err := s.pool.QueryRow(ctx, `
		UPDATE jobs SET lease_expires_at = now() + make_interval(secs => $4)
		WHERE id::text = $1 AND lease_id = $2 AND runner_id = $3 AND status IN ('leased','running')
		RETURNING status
	`, jobID, leaseID, runnerID, ttl.Seconds()).Scan(&status)
	if err == nil {
		_, _ = s.pool.Exec(ctx, `UPDATE runners SET last_heartbeat_at = now() WHERE id = $1`, runnerID)
		return status, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	err = s.pool.QueryRow(ctx, `
		SELECT status FROM jobs WHERE id::text = $1 AND lease_id = $2
	`, jobID, leaseID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", identity.ErrForbidden
	}
	if err != nil {
		return "", err
	}
	return status, nil
}

func (s *Service) CompleteJob(ctx context.Context, jobID, leaseID, status, errMsg string) (string, error) {
	if status != "succeeded" && status != "failed" && status != "canceled" {
		return "", fmt.Errorf("%w: invalid status", identity.ErrInvalidInput)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	var runID, runnerID string
	err = tx.QueryRow(ctx, `
		UPDATE jobs
		SET status = $3, error_message = $4, finished_at = now(), lease_expires_at = NULL
		WHERE id = $1 AND lease_id = $2 AND status IN ('leased','running')
		RETURNING run_id, COALESCE(runner_id::text, '')
	`, jobID, leaseID, status, errMsg).Scan(&runID, &runnerID)
	if errors.Is(err, pgx.ErrNoRows) {
		var existing string
		err = tx.QueryRow(ctx, `
			SELECT status, run_id::text FROM jobs WHERE id::text = $1 AND lease_id = $2
		`, jobID, leaseID).Scan(&existing, &runID)
		if err != nil {
			return "", identity.ErrForbidden
		}
		if existing == "succeeded" || existing == "failed" || existing == "canceled" {
			return runID, nil
		}
		return "", identity.ErrForbidden
	}
	if err != nil {
		return "", err
	}
	if runnerID != "" {
		if _, err := tx.Exec(ctx, `
			UPDATE runners SET status = CASE WHEN drained THEN 'draining' ELSE 'idle' END WHERE id = $1
		`, runnerID); err != nil {
			return "", err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return runID, nil
}

func (s *Service) AppendLog(ctx context.Context, runnerID, jobID, stepID, stream, line string) error {
	if err := s.requireJobRunner(ctx, jobID, runnerID); err != nil {
		return err
	}
	if stream == "" {
		stream = "stdout"
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, jobID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO job_logs (job_id, step_id, seq, stream, line)
		SELECT $1, NULLIF($2,'')::uuid, COALESCE(MAX(seq), 0) + 1, $3, $4
		FROM job_logs WHERE job_id = $1
	`, jobID, stepID, stream, line); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) ListLogs(ctx context.Context, projectID, jobID string, afterSeq int64) ([]map[string]any, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT l.seq, l.stream, l.line, l.created_at
		FROM job_logs l
		JOIN jobs j ON j.id = l.job_id
		JOIN pipeline_runs r ON r.id = j.run_id
		WHERE l.job_id::text = $1 AND r.project_id = $2 AND l.seq > $3
		ORDER BY l.seq ASC LIMIT 1000
	`, jobID, projectID, afterSeq)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var seq int64
		var stream, line string
		var created time.Time
		if err := rows.Scan(&seq, &stream, &line, &created); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"seq": seq, "stream": stream, "line": line, "created_at": created})
	}
	return out, rows.Err()
}

func (s *Service) UpdateStepStatus(ctx context.Context, runnerID, stepID, status string, exitCode *int) error {
	switch status {
	case "pending", "running", "succeeded", "failed", "canceled", "skipped":
	default:
		return fmt.Errorf("%w: invalid status", identity.ErrInvalidInput)
	}
	ct, err := s.pool.Exec(ctx, `
		UPDATE job_steps
		SET status = $2,
		    exit_code = $3,
		    started_at = CASE WHEN $2 = 'running' THEN COALESCE(started_at, now()) ELSE started_at END,
		    finished_at = CASE WHEN $2 IN ('succeeded','failed','canceled','skipped') THEN now() ELSE finished_at END
		WHERE id = $1 AND job_id IN (SELECT id FROM jobs WHERE runner_id = $4 AND status IN ('leased','running'))
	`, stepID, status, exitCode, runnerID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return identity.ErrForbidden
	}
	return nil
}

func (s *Service) ExpireLeases(ctx context.Context) (int64, []string, error) {
	requeued, err := s.pool.Query(ctx, `
		UPDATE jobs j
		SET status = 'queued', runner_id = NULL, lease_id = NULL, lease_expires_at = NULL, error_message = 'lease expired'
		FROM (SELECT id, runner_id FROM jobs WHERE status = 'leased' AND lease_expires_at < now()) prev
		WHERE j.id = prev.id
		RETURNING COALESCE(prev.runner_id::text, '')
	`)
	if err != nil {
		return 0, nil, err
	}
	defer requeued.Close()
	var requeuedCount int64
	var freedRunners []string
	for requeued.Next() {
		var runnerID string
		if err := requeued.Scan(&runnerID); err != nil {
			return 0, nil, err
		}
		requeuedCount++
		if runnerID != "" {
			freedRunners = append(freedRunners, runnerID)
		}
	}
	requeued.Close()
	if err := requeued.Err(); err != nil {
		return 0, nil, err
	}
	if err := s.resetBusyRunners(ctx, freedRunners); err != nil {
		return requeuedCount, nil, err
	}
	rows, err := s.pool.Query(ctx, `
		UPDATE jobs
		SET status = 'failed', error_message = 'runner lost: lease expired', finished_at = now(), lease_id = NULL, lease_expires_at = NULL
		WHERE status = 'running' AND lease_expires_at IS NOT NULL AND lease_expires_at < now()
		RETURNING run_id::text, COALESCE(runner_id::text, '')
	`)
	if err != nil {
		return requeuedCount, nil, err
	}
	defer rows.Close()
	seenRuns := map[string]struct{}{}
	var lostRunIDs []string
	var lostRunners []string
	for rows.Next() {
		var runID, runnerID string
		if err := rows.Scan(&runID, &runnerID); err != nil {
			return requeuedCount, nil, err
		}
		if _, dup := seenRuns[runID]; !dup {
			seenRuns[runID] = struct{}{}
			lostRunIDs = append(lostRunIDs, runID)
		}
		if runnerID != "" {
			lostRunners = append(lostRunners, runnerID)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return requeuedCount, nil, err
	}
	if err := s.resetBusyRunners(ctx, lostRunners); err != nil {
		return requeuedCount, lostRunIDs, err
	}
	return requeuedCount, lostRunIDs, nil
}

func (s *Service) resetBusyRunners(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE runners SET status = CASE WHEN drained THEN 'draining' ELSE 'idle' END
		WHERE id = ANY($1) AND status = 'busy'
	`, ids)
	return err
}

func lowerLabels(labels []string) []string {
	out := make([]string, 0, len(labels))
	for _, l := range labels {
		l = strings.ToLower(strings.TrimSpace(l))
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

func nullIfEmpty(v string) any {
	if v == "" {
		return nil
	}
	return v
}
