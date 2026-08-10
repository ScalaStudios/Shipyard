package scm

import (
	"context"
	"errors"
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
	InstallationID string    `json:"installation_id,omitempty"`
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
	InstallationID string
	ActorID        string
}

const KindGitHubApp = "github_app_install"

func (s *Service) SetInstallationTokenFunc(fn func(ctx context.Context, installationID string) (string, error)) {
	s.installationToken = fn
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
	if token == "" && strings.TrimSpace(in.InstallationID) == "" {
		return ForgeCredential{}, fmt.Errorf("%w: access_token required", identity.ErrInvalidInput)
	}
	base := strings.TrimRight(strings.TrimSpace(in.BaseURL), "/")
	if base == "" {
		base = DefaultBaseURL(provider)
	}
	if base == "" {
		return ForgeCredential{}, fmt.Errorf("%w: base_url required", identity.ErrInvalidInput)
	}
	sealed, err := s.secrets.SealString(token)
	if err != nil {
		return ForgeCredential{}, err
	}
	var c ForgeCredential
	err = s.pool.QueryRow(ctx, `
		INSERT INTO forge_credentials (organization_id, provider, kind, name, base_url, access_token, installation_id, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, organization_id, provider, kind, name, base_url, created_at, updated_at,
		          access_token <> '', COALESCE(created_by::text,'')
	`, in.OrganizationID, provider, kind, name, base, sealed, strings.TrimSpace(in.InstallationID), nullIfEmpty(in.ActorID),
	).Scan(&c.ID, &c.OrganizationID, &c.Provider, &c.Kind, &c.Name, &c.BaseURL, &c.CreatedAt, &c.UpdatedAt, &c.HasToken, &c.CreatedBy)
	if err != nil && strings.Contains(err.Error(), "SQLSTATE 23505") {
		return ForgeCredential{}, identity.ErrConflict
	}
	return c, err
}

func (s *Service) UpsertCredential(ctx context.Context, in CreateCredentialInput) (ForgeCredential, error) {
	c, err := s.CreateCredential(ctx, in)
	if !errors.Is(err, identity.ErrConflict) {
		return c, err
	}
	base := strings.TrimRight(strings.TrimSpace(in.BaseURL), "/")
	if base == "" {
		base = DefaultBaseURL(NormalizeProvider(in.Provider))
	}
	sealed, err := s.secrets.SealString(strings.TrimSpace(in.AccessToken))
	if err != nil {
		return ForgeCredential{}, err
	}
	err = s.pool.QueryRow(ctx, `
		UPDATE forge_credentials
		SET provider = $3, kind = $4, base_url = $5, access_token = $6, installation_id = $7, updated_at = now()
		WHERE organization_id = $1 AND name = $2
		RETURNING id, organization_id, provider, kind, name, base_url, created_at, updated_at,
		          access_token <> '', COALESCE(created_by::text,'')
	`, in.OrganizationID, strings.ToLower(strings.TrimSpace(in.Name)), NormalizeProvider(in.Provider),
		strings.ToLower(strings.TrimSpace(in.Kind)), base, sealed, strings.TrimSpace(in.InstallationID),
	).Scan(&c.ID, &c.OrganizationID, &c.Provider, &c.Kind, &c.Name, &c.BaseURL, &c.CreatedAt, &c.UpdatedAt, &c.HasToken, &c.CreatedBy)
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
		SELECT id, organization_id, provider, kind, name, base_url, access_token, installation_id, created_at, updated_at,
		       access_token <> '', COALESCE(created_by::text,'')
		FROM forge_credentials WHERE organization_id = $1 AND id = $2
	`, orgID, id).Scan(
		&c.ID, &c.OrganizationID, &c.Provider, &c.Kind, &c.Name, &c.BaseURL, &c.AccessToken, &c.InstallationID, &c.CreatedAt, &c.UpdatedAt, &c.HasToken, &c.CreatedBy,
	)
	if err != nil {
		return ForgeCredential{}, identity.ErrNotFound
	}
	if c.Kind == KindGitHubApp && c.InstallationID != "" {
		if s.installationToken == nil {
			return ForgeCredential{}, fmt.Errorf("github app is not configured on this instance")
		}
		minted, err := s.installationToken(ctx, c.InstallationID)
		if err != nil {
			return ForgeCredential{}, fmt.Errorf("mint installation token for %s: %w", c.Name, err)
		}
		c.AccessToken = minted
		c.HasToken = true
		return c, nil
	}
	token, err := s.secrets.OpenString(c.AccessToken)
	if err != nil {
		return ForgeCredential{}, fmt.Errorf("decrypt credential %s: %w", c.Name, err)
	}
	c.AccessToken = token
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
