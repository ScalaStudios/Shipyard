package scm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/identity"
)

type Connection struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	ProjectID      string    `json:"project_id"`
	Provider       string    `json:"provider"`
	Name           string    `json:"name"`
	BaseURL        string    `json:"base_url"`
	RepoOwner      string    `json:"repo_owner"`
	RepoName       string    `json:"repo_name"`
	BotUsername    string    `json:"bot_username"`
	PipelineSlug   string    `json:"pipeline_slug"`
	Enabled        bool      `json:"enabled"`
	HasToken       bool      `json:"has_token"`
	HasSecret      bool      `json:"has_webhook_secret"`
	CreatedBy      string    `json:"created_by,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	AccessToken    string    `json:"-"`
	WebhookSecret  string    `json:"-"`
}

type Service struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

type CreateInput struct {
	OrganizationID string
	ProjectID      string
	Provider       string
	Name           string
	BaseURL        string
	RepoOwner      string
	RepoName       string
	AccessToken    string
	BotUsername    string
	WebhookSecret  string
	PipelineSlug   string
	ActorID        string
}

func (s *Service) Create(ctx context.Context, in CreateInput) (Connection, error) {
	provider := NormalizeProvider(in.Provider)
	if !ValidProvider(provider) {
		return Connection{}, fmt.Errorf("%w: unknown provider", identity.ErrInvalidInput)
	}
	name := strings.ToLower(strings.TrimSpace(in.Name))
	if name == "" {
		return Connection{}, fmt.Errorf("%w: name required", identity.ErrInvalidInput)
	}
	base := strings.TrimRight(strings.TrimSpace(in.BaseURL), "/")
	if base == "" {
		base = DefaultBaseURL(provider)
	}
	if base == "" {
		return Connection{}, fmt.Errorf("%w: base_url required", identity.ErrInvalidInput)
	}
	bot := strings.TrimSpace(in.BotUsername)
	if bot == "" {
		bot = "shipyard[bot]"
	}
	var c Connection
	err := s.pool.QueryRow(ctx, `
		INSERT INTO scm_connections (
			organization_id, project_id, provider, name, base_url, repo_owner, repo_name,
			access_token, bot_username, webhook_secret, pipeline_slug, created_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING id, organization_id, project_id, provider, name, base_url, repo_owner, repo_name,
		          bot_username, pipeline_slug, enabled, created_at, updated_at,
		          access_token <> '', webhook_secret <> '', COALESCE(created_by::text,'')
	`, in.OrganizationID, in.ProjectID, provider, name, base, strings.TrimSpace(in.RepoOwner),
		strings.TrimSpace(in.RepoName), in.AccessToken, bot, in.WebhookSecret, strings.TrimSpace(in.PipelineSlug), nullIfEmpty(in.ActorID),
	).Scan(
		&c.ID, &c.OrganizationID, &c.ProjectID, &c.Provider, &c.Name, &c.BaseURL, &c.RepoOwner, &c.RepoName,
		&c.BotUsername, &c.PipelineSlug, &c.Enabled, &c.CreatedAt, &c.UpdatedAt, &c.HasToken, &c.HasSecret, &c.CreatedBy,
	)
	if err != nil && strings.Contains(err.Error(), "SQLSTATE 23505") {
		return Connection{}, identity.ErrConflict
	}
	return c, err
}

func (s *Service) List(ctx context.Context, projectID string) ([]Connection, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, organization_id, project_id, provider, name, base_url, repo_owner, repo_name,
		       bot_username, pipeline_slug, enabled, created_at, updated_at,
		       access_token <> '', webhook_secret <> ''
		FROM scm_connections WHERE project_id = $1 ORDER BY name
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Connection
	for rows.Next() {
		var c Connection
		if err := rows.Scan(
			&c.ID, &c.OrganizationID, &c.ProjectID, &c.Provider, &c.Name, &c.BaseURL, &c.RepoOwner, &c.RepoName,
			&c.BotUsername, &c.PipelineSlug, &c.Enabled, &c.CreatedAt, &c.UpdatedAt, &c.HasToken, &c.HasSecret,
		); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Service) Get(ctx context.Context, projectID, id string) (Connection, error) {
	var c Connection
	err := s.pool.QueryRow(ctx, `
		SELECT id, organization_id, project_id, provider, name, base_url, repo_owner, repo_name,
		       access_token, webhook_secret, bot_username, pipeline_slug, enabled, created_at, updated_at,
		       access_token <> '', webhook_secret <> '', COALESCE(created_by::text,'')
		FROM scm_connections WHERE project_id = $1 AND id = $2
	`, projectID, id).Scan(
		&c.ID, &c.OrganizationID, &c.ProjectID, &c.Provider, &c.Name, &c.BaseURL, &c.RepoOwner, &c.RepoName,
		&c.AccessToken, &c.WebhookSecret, &c.BotUsername, &c.PipelineSlug, &c.Enabled, &c.CreatedAt, &c.UpdatedAt,
		&c.HasToken, &c.HasSecret, &c.CreatedBy,
	)
	if err == pgx.ErrNoRows {
		return Connection{}, identity.ErrNotFound
	}
	return c, err
}

func (s *Service) GetByID(ctx context.Context, id string) (Connection, error) {
	var c Connection
	err := s.pool.QueryRow(ctx, `
		SELECT id, organization_id, project_id, provider, name, base_url, repo_owner, repo_name,
		       access_token, webhook_secret, bot_username, pipeline_slug, enabled, created_at, updated_at,
		       access_token <> '', webhook_secret <> '', COALESCE(created_by::text,'')
		FROM scm_connections WHERE id = $1
	`, id).Scan(
		&c.ID, &c.OrganizationID, &c.ProjectID, &c.Provider, &c.Name, &c.BaseURL, &c.RepoOwner, &c.RepoName,
		&c.AccessToken, &c.WebhookSecret, &c.BotUsername, &c.PipelineSlug, &c.Enabled, &c.CreatedAt, &c.UpdatedAt,
		&c.HasToken, &c.HasSecret, &c.CreatedBy,
	)
	if err == pgx.ErrNoRows {
		return Connection{}, identity.ErrNotFound
	}
	return c, err
}

func (s *Service) FindForWebhook(ctx context.Context, provider, projectID, connectionID string) (Connection, error) {
	if connectionID != "" {
		return s.GetByID(ctx, connectionID)
	}
	provider = NormalizeProvider(provider)
	rows, err := s.pool.Query(ctx, `
		SELECT id FROM scm_connections
		WHERE enabled AND provider = $1 AND ($2 = '' OR project_id::text = $2)
		ORDER BY created_at ASC LIMIT 2
	`, provider, projectID)
	if err != nil {
		return Connection{}, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return Connection{}, err
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return Connection{}, identity.ErrNotFound
	}
	if len(ids) > 1 && projectID == "" {
		return Connection{}, fmt.Errorf("%w: ambiguous connection; pass connection_id or project_id", identity.ErrInvalidInput)
	}
	return s.GetByID(ctx, ids[0])
}

func (s *Service) Delete(ctx context.Context, projectID, id string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM scm_connections WHERE project_id = $1 AND id = $2`, projectID, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return identity.ErrNotFound
	}
	return nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
