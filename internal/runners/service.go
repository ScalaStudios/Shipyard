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
		RETURNING id, name, labels, capabilities, status, last_heartbeat_at, drained, created_at
	`, name, auth.HashToken(runnerToken), auth.TokenPrefix(runnerToken), labels, capabilities, orgID).
		Scan(&r.ID, &r.Name, &r.Labels, &r.Capabilities, &r.Status, &r.LastHeartbeatAt, &r.Drained, &r.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `
			INSERT INTO runners (name, token_hash, token_prefix, labels, capabilities, status, organization_id, last_heartbeat_at)
			VALUES ($1,$2,$3,$4,$5,'idle',$6,now())
			RETURNING id, name, labels, capabilities, status, last_heartbeat_at, drained, created_at
		`, name, auth.HashToken(runnerToken), auth.TokenPrefix(runnerToken), labels, capabilities, orgID).
			Scan(&r.ID, &r.Name, &r.Labels, &r.Capabilities, &r.Status, &r.LastHeartbeatAt, &r.Drained, &r.CreatedAt)
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
		SELECT id, name, labels, capabilities, status, last_heartbeat_at, drained, created_at
		FROM runners WHERE token_hash = $1
	`, auth.HashToken(token)).Scan(&r.ID, &r.Name, &r.Labels, &r.Capabilities, &r.Status, &r.LastHeartbeatAt, &r.Drained, &r.CreatedAt)
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
		SELECT id, name, labels, capabilities, status, last_heartbeat_at, drained, created_at
		FROM runners ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Runner
	for rows.Next() {
		var r Runner
		if err := rows.Scan(&r.ID, &r.Name, &r.Labels, &r.Capabilities, &r.Status, &r.LastHeartbeatAt, &r.Drained, &r.CreatedAt); err != nil {
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

	rows, err := tx.Query(ctx, `
		SELECT id, run_id, name, runner_labels
		FROM jobs
		WHERE status = 'queued'
		ORDER BY queued_at NULLS FIRST, created_at
		FOR UPDATE SKIP LOCKED
		LIMIT 20
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var selected *LeaseJob
	for rows.Next() {
		var id, runID, name string
		var labels []string
		if err := rows.Scan(&id, &runID, &name, &labels); err != nil {
			return nil, err
		}
		if labelsMatch(runner.Labels, labels) {
			selected = &LeaseJob{ID: id, RunID: runID, Name: name, RunnerLabels: labels}
			break
		}
	}
	rows.Close()
	if selected == nil {
		return nil, tx.Commit(ctx)
	}

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

func (s *Service) GetJobSteps(ctx context.Context, jobID string) ([]Step, error) {
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
		UPDATE jobs SET status = 'running', started_at = COALESCE(started_at, now())
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

func (s *Service) AppendLog(ctx context.Context, jobID, stepID, stream, line string) error {
	if stream == "" {
		stream = "stdout"
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO job_logs (job_id, step_id, seq, stream, line)
		SELECT $1, NULLIF($2,'')::uuid, COALESCE(MAX(seq), 0) + 1, $3, $4
		FROM job_logs WHERE job_id = $1
	`, jobID, stepID, stream, line)
	return err
}

func (s *Service) ListLogs(ctx context.Context, jobID string, afterSeq int64) ([]map[string]any, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT seq, stream, line, created_at
		FROM job_logs WHERE job_id = $1 AND seq > $2
		ORDER BY seq ASC LIMIT 1000
	`, jobID, afterSeq)
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

func (s *Service) UpdateStepStatus(ctx context.Context, stepID, status string, exitCode *int) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE job_steps
		SET status = $2,
		    exit_code = $3,
		    started_at = CASE WHEN $2 = 'running' THEN COALESCE(started_at, now()) ELSE started_at END,
		    finished_at = CASE WHEN $2 IN ('succeeded','failed','canceled','skipped') THEN now() ELSE finished_at END
		WHERE id = $1
	`, stepID, status, exitCode)
	return err
}

func (s *Service) ExpireLeases(ctx context.Context) (int64, error) {
	ct, err := s.pool.Exec(ctx, `
		UPDATE jobs
		SET status = 'queued', runner_id = NULL, lease_id = NULL, lease_expires_at = NULL, error_message = 'lease expired'
		WHERE status = 'leased' AND lease_expires_at < now()
	`)
	if err != nil {
		return 0, err
	}
	return ct.RowsAffected(), nil
}

func labelsMatch(runnerLabels, required []string) bool {
	set := map[string]struct{}{}
	for _, l := range runnerLabels {
		set[strings.ToLower(l)] = struct{}{}
	}
	for _, need := range required {
		if _, ok := set[strings.ToLower(need)]; !ok {
			return false
		}
	}
	return true
}

func nullIfEmpty(v string) any {
	if v == "" {
		return nil
	}
	return v
}
