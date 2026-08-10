package scm

import (
	"context"
	"encoding/json"
	"time"
)

type Delivery struct {
	ID             string          `json:"id"`
	ConnectionID   string          `json:"connection_id,omitempty"`
	OrganizationID string          `json:"organization_id,omitempty"`
	ProjectID      string          `json:"project_id,omitempty"`
	Provider       string          `json:"provider"`
	EventType      string          `json:"event_type"`
	DeliveryID     string          `json:"delivery_id"`
	Status         string          `json:"status"`
	RunID          string          `json:"run_id,omitempty"`
	ErrorMessage   string          `json:"error_message,omitempty"`
	Summary        string          `json:"summary"`
	CreatedAt      time.Time       `json:"created_at"`
	Payload        json.RawMessage `json:"-"`
}

type DeliveryInput struct {
	ConnectionID   string
	OrganizationID string
	ProjectID      string
	Provider       string
	EventType      string
	DeliveryID     string
	Status         string
	RunID          string
	ErrorMessage   string
	Summary        string
	Payload        json.RawMessage
}

func (s *Service) RecordDelivery(ctx context.Context, in DeliveryInput) (Delivery, error) {
	if in.Status == "" {
		in.Status = "accepted"
	}
	if in.Payload == nil {
		in.Payload = json.RawMessage(`{}`)
	}
	var d Delivery
	err := s.pool.QueryRow(ctx, `
		INSERT INTO webhook_deliveries (
			connection_id, organization_id, project_id, provider, event_type, delivery_id,
			status, run_id, error_message, summary, payload
		) VALUES (
			NULLIF($1,'')::uuid, NULLIF($2,'')::uuid, NULLIF($3,'')::uuid, $4, $5, $6,
			$7, NULLIF($8,'')::uuid, $9, $10, $11::jsonb
		)
		RETURNING id, COALESCE(connection_id::text,''), COALESCE(organization_id::text,''), COALESCE(project_id::text,''),
		          provider, event_type, delivery_id, status, COALESCE(run_id::text,''), error_message, summary, created_at
	`, in.ConnectionID, in.OrganizationID, in.ProjectID, in.Provider, in.EventType, in.DeliveryID,
		in.Status, in.RunID, in.ErrorMessage, in.Summary, string(in.Payload),
	).Scan(&d.ID, &d.ConnectionID, &d.OrganizationID, &d.ProjectID, &d.Provider, &d.EventType, &d.DeliveryID,
		&d.Status, &d.RunID, &d.ErrorMessage, &d.Summary, &d.CreatedAt)
	return d, err
}

func (s *Service) ListDeliveries(ctx context.Context, projectID string, limit int) ([]Delivery, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, COALESCE(connection_id::text,''), COALESCE(organization_id::text,''), COALESCE(project_id::text,''),
		       provider, event_type, delivery_id, status, COALESCE(run_id::text,''), error_message, summary, created_at
		FROM webhook_deliveries
		WHERE project_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Delivery
	for rows.Next() {
		var d Delivery
		if err := rows.Scan(&d.ID, &d.ConnectionID, &d.OrganizationID, &d.ProjectID, &d.Provider, &d.EventType,
			&d.DeliveryID, &d.Status, &d.RunID, &d.ErrorMessage, &d.Summary, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Service) AttachRunSCM(ctx context.Context, runID, connectionID string, prNumber int, commentID, targetURL string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE pipeline_runs
		SET scm_connection_id = NULLIF($2,'')::uuid,
		    scm_pr_number = $3,
		    scm_comment_id = $4,
		    scm_target_url = $5
		WHERE id = $1
	`, runID, connectionID, prNumber, commentID, targetURL)
	return err
}

type RunSCM struct {
	RunID          string
	Number         int64
	Status         string
	GitRef         string
	GitSHA         string
	OrganizationID string
	ProjectID      string
	ConnectionID   string
	PRNumber       int
	CommentID      string
	TargetURL      string
}

func (s *Service) GetRunSCM(ctx context.Context, runID string) (RunSCM, error) {
	var r RunSCM
	err := s.pool.QueryRow(ctx, `
		SELECT id, number, status, git_ref, git_sha, organization_id, project_id,
		       COALESCE(scm_connection_id::text,''), scm_pr_number, scm_comment_id, scm_target_url
		FROM pipeline_runs WHERE id = $1
	`, runID).Scan(&r.RunID, &r.Number, &r.Status, &r.GitRef, &r.GitSHA, &r.OrganizationID, &r.ProjectID,
		&r.ConnectionID, &r.PRNumber, &r.CommentID, &r.TargetURL)
	return r, err
}
