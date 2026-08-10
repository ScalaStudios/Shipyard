package scm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/identity"
)

type ForgeCredential struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	Provider       string    `json:"provider"`
	Kind           string    `json:"kind"`
	Name           string    `json:"name"`
	BaseURL        string    `json:"base_url"`
	HasToken       bool      `json:"has_token"`
	CreatedBy      string    `json:"created_by,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	AccessToken    string    `json:"-"`
}

type CreateCredentialInput struct {
	OrganizationID string
	Provider       string
	Kind           string
	Name           string
	BaseURL        string
	AccessToken    string
	ActorID        string
}

func (s *Service) CreateCredential(ctx context.Context, in CreateCredentialInput) (ForgeCredential, error) {
	provider := NormalizeProvider(in.Provider)
	if !ValidProvider(provider) {
		return ForgeCredential{}, fmt.Errorf("%w: unknown provider", identity.ErrInvalidInput)
	}
	kind := strings.ToLower(strings.TrimSpace(in.Kind))
	if kind == "" {
		kind = "pat"
	}
	name := strings.ToLower(strings.TrimSpace(in.Name))
	if name == "" {
		return ForgeCredential{}, fmt.Errorf("%w: name required", identity.ErrInvalidInput)
	}
	token := strings.TrimSpace(in.AccessToken)
	if token == "" {
		return ForgeCredential{}, fmt.Errorf("%w: access_token required", identity.ErrInvalidInput)
	}
	base := strings.TrimRight(strings.TrimSpace(in.BaseURL), "/")
	if base == "" {
		base = DefaultBaseURL(provider)
	}
	if base == "" {
		return ForgeCredential{}, fmt.Errorf("%w: base_url required", identity.ErrInvalidInput)
	}
	var c ForgeCredential
	err := s.pool.QueryRow(ctx, `
		INSERT INTO forge_credentials (organization_id, provider, kind, name, base_url, access_token, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, organization_id, provider, kind, name, base_url, created_at, updated_at,
		          access_token <> '', COALESCE(created_by::text,'')
	`, in.OrganizationID, provider, kind, name, base, token, nullIfEmpty(in.ActorID),
	).Scan(&c.ID, &c.OrganizationID, &c.Provider, &c.Kind, &c.Name, &c.BaseURL, &c.CreatedAt, &c.UpdatedAt, &c.HasToken, &c.CreatedBy)
	if err != nil && strings.Contains(err.Error(), "SQLSTATE 23505") {
		return ForgeCredential{}, identity.ErrConflict
	}
	return c, err
}

func (s *Service) ListCredentials(ctx context.Context, orgID string) ([]ForgeCredential, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, organization_id, provider, kind, name, base_url, created_at, updated_at,
		       access_token <> '', COALESCE(created_by::text,'')
		FROM forge_credentials WHERE organization_id = $1 ORDER BY name
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ForgeCredential
	for rows.Next() {
		var c ForgeCredential
		if err := rows.Scan(&c.ID, &c.OrganizationID, &c.Provider, &c.Kind, &c.Name, &c.BaseURL, &c.CreatedAt, &c.UpdatedAt, &c.HasToken, &c.CreatedBy); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Service) GetCredential(ctx context.Context, orgID, id string) (ForgeCredential, error) {
	var c ForgeCredential
	err := s.pool.QueryRow(ctx, `
		SELECT id, organization_id, provider, kind, name, base_url, access_token, created_at, updated_at,
		       access_token <> '', COALESCE(created_by::text,'')
		FROM forge_credentials WHERE organization_id = $1 AND id = $2
	`, orgID, id).Scan(
		&c.ID, &c.OrganizationID, &c.Provider, &c.Kind, &c.Name, &c.BaseURL, &c.AccessToken, &c.CreatedAt, &c.UpdatedAt, &c.HasToken, &c.CreatedBy,
	)
	if err != nil {
		return ForgeCredential{}, identity.ErrNotFound
	}
	return c, nil
}

func (s *Service) DeleteCredential(ctx context.Context, orgID, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM forge_credentials WHERE organization_id = $1 AND id = $2`, orgID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return identity.ErrNotFound
	}
	return nil
}
