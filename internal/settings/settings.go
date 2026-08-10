package settings

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/identity"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/oidc"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/secrets"
)

const (
	PurposeLogin = "login"
	PurposeForge = "forge"
)

type Provider struct {
	ID           string    `json:"id"`
	Purpose      string    `json:"purpose"`
	Name         string    `json:"name"`
	Kind         string    `json:"kind"`
	Issuer       string    `json:"issuer"`
	ClientID     string    `json:"client_id"`
	RedirectURL  string    `json:"redirect_url"`
	Scopes       []string  `json:"scopes"`
	Enabled      bool      `json:"enabled"`
	HasSecret    bool      `json:"has_client_secret"`
	UpdatedAt    time.Time `json:"updated_at"`
	ClientSecret string    `json:"-"`
}

type Setting struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	IsSecret  bool      `json:"is_secret"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Service struct {
	pool    *pgxpool.Pool
	secrets *secrets.Box

	mu        sync.RWMutex
	version   time.Time
	byPurpose map[string][]oidc.ProviderConfig
}

func New(pool *pgxpool.Pool, box *secrets.Box) *Service {
	return &Service{pool: pool, secrets: box, byPurpose: map[string][]oidc.ProviderConfig{}}
}

func (s *Service) Get(ctx context.Context, key string) (string, error) {
	var value string
	var isSecret bool
	err := s.pool.QueryRow(ctx, `SELECT value, is_secret FROM instance_settings WHERE key = $1`, key).Scan(&value, &isSecret)
	if err != nil {
		return "", identity.ErrNotFound
	}
	if !isSecret {
		return value, nil
	}
	return s.secrets.OpenString(value)
}

func (s *Service) List(ctx context.Context) ([]Setting, error) {
	rows, err := s.pool.Query(ctx, `SELECT key, value, is_secret, updated_at FROM instance_settings ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Setting{}
	for rows.Next() {
		var item Setting
		if err := rows.Scan(&item.Key, &item.Value, &item.IsSecret, &item.UpdatedAt); err != nil {
			return nil, err
		}
		if item.IsSecret {
			item.Value = ""
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Service) Set(ctx context.Context, key, value string, isSecret bool, actorID string) error {
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "" {
		return fmt.Errorf("%w: key required", identity.ErrInvalidInput)
	}
	stored := value
	if isSecret {
		sealed, err := s.secrets.SealString(value)
		if err != nil {
			return err
		}
		stored = sealed
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO instance_settings (key, value, is_secret, updated_by, updated_at)
		VALUES ($1, $2, $3, NULLIF($4,'')::uuid, now())
		ON CONFLICT (key) DO UPDATE
		SET value = EXCLUDED.value, is_secret = EXCLUDED.is_secret,
		    updated_by = EXCLUDED.updated_by, updated_at = now()
	`, key, stored, isSecret, actorID)
	return err
}

func (s *Service) ListProviders(ctx context.Context, purpose string) ([]Provider, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, purpose, name, kind, issuer, client_id, client_secret, redirect_url, scopes, enabled, updated_at
		FROM auth_providers WHERE purpose = $1 ORDER BY name
	`, purpose)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Provider{}
	for rows.Next() {
		var p Provider
		if err := rows.Scan(&p.ID, &p.Purpose, &p.Name, &p.Kind, &p.Issuer, &p.ClientID,
			&p.ClientSecret, &p.RedirectURL, &p.Scopes, &p.Enabled, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.HasSecret = p.ClientSecret != ""
		p.ClientSecret = ""
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Service) UpsertProvider(ctx context.Context, p Provider, actorID string) (Provider, error) {
	p.Purpose = strings.ToLower(strings.TrimSpace(p.Purpose))
	if p.Purpose != PurposeLogin && p.Purpose != PurposeForge {
		return Provider{}, fmt.Errorf("%w: purpose must be login or forge", identity.ErrInvalidInput)
	}
	p.Name = strings.ToLower(strings.TrimSpace(p.Name))
	if p.Name == "" {
		return Provider{}, fmt.Errorf("%w: name required", identity.ErrInvalidInput)
	}
	if strings.TrimSpace(p.ClientID) == "" {
		return Provider{}, fmt.Errorf("%w: client_id required", identity.ErrInvalidInput)
	}

	sealed := ""
	if p.ClientSecret != "" {
		var err error
		sealed, err = s.secrets.SealString(p.ClientSecret)
		if err != nil {
			return Provider{}, err
		}
	}

	var out Provider
	err := s.pool.QueryRow(ctx, `
		INSERT INTO auth_providers (purpose, name, kind, issuer, client_id, client_secret, redirect_url, scopes, enabled, updated_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NULLIF($10,'')::uuid)
		ON CONFLICT (purpose, name) DO UPDATE SET
			kind = EXCLUDED.kind,
			issuer = EXCLUDED.issuer,
			client_id = EXCLUDED.client_id,
			client_secret = CASE WHEN EXCLUDED.client_secret = '' THEN auth_providers.client_secret ELSE EXCLUDED.client_secret END,
			redirect_url = EXCLUDED.redirect_url,
			scopes = EXCLUDED.scopes,
			enabled = EXCLUDED.enabled,
			updated_by = EXCLUDED.updated_by,
			updated_at = now()
		RETURNING id, purpose, name, kind, issuer, client_id, client_secret <> '', redirect_url, scopes, enabled, updated_at
	`, p.Purpose, p.Name, strings.ToLower(strings.TrimSpace(p.Kind)), strings.TrimRight(strings.TrimSpace(p.Issuer), "/"),
		strings.TrimSpace(p.ClientID), sealed, strings.TrimSpace(p.RedirectURL), normalizeScopes(p.Scopes), p.Enabled, actorID,
	).Scan(&out.ID, &out.Purpose, &out.Name, &out.Kind, &out.Issuer, &out.ClientID, &out.HasSecret,
		&out.RedirectURL, &out.Scopes, &out.Enabled, &out.UpdatedAt)
	if err != nil {
		return Provider{}, err
	}
	s.invalidate()
	return out, nil
}

func (s *Service) DeleteProvider(ctx context.Context, purpose, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM auth_providers WHERE purpose = $1 AND id = $2`, purpose, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return identity.ErrNotFound
	}
	s.invalidate()
	return nil
}

func (s *Service) OIDCConfigs(ctx context.Context, purpose string) []oidc.ProviderConfig {
	stamp := s.remoteVersion(ctx)
	s.mu.RLock()
	cached, ok := s.byPurpose[purpose]
	fresh := ok && stamp.Equal(s.version)
	s.mu.RUnlock()
	if fresh {
		return cached
	}

	configs, err := s.loadConfigs(ctx, purpose)
	if err != nil {
		s.mu.RLock()
		defer s.mu.RUnlock()
		return s.byPurpose[purpose]
	}
	s.mu.Lock()
	if !stamp.Equal(s.version) {
		s.byPurpose = map[string][]oidc.ProviderConfig{}
		s.version = stamp
	}
	s.byPurpose[purpose] = configs
	s.mu.Unlock()
	return configs
}

func (s *Service) loadConfigs(ctx context.Context, purpose string) ([]oidc.ProviderConfig, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT name, kind, issuer, client_id, client_secret, redirect_url, scopes
		FROM auth_providers WHERE purpose = $1 AND enabled ORDER BY name
	`, purpose)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []oidc.ProviderConfig
	for rows.Next() {
		var name, kind, issuer, clientID, clientSecret, redirect string
		var scopes []string
		if err := rows.Scan(&name, &kind, &issuer, &clientID, &clientSecret, &redirect, &scopes); err != nil {
			return nil, err
		}
		plain, err := s.secrets.OpenString(clientSecret)
		if err != nil {
			return nil, fmt.Errorf("decrypt %s provider %s: %w", purpose, name, err)
		}
		out = append(out, oidc.ProviderConfig{
			Name:         name,
			Kind:         oidc.ProviderKind(kind),
			Issuer:       issuer,
			ClientID:     clientID,
			ClientSecret: plain,
			RedirectURL:  redirect,
			Scopes:       scopes,
		})
	}
	return out, rows.Err()
}

func (s *Service) remoteVersion(ctx context.Context) time.Time {
	var stamp *time.Time
	if err := s.pool.QueryRow(ctx, `SELECT max(updated_at) FROM auth_providers`).Scan(&stamp); err != nil || stamp == nil {
		return time.Time{}
	}
	return *stamp
}

func (s *Service) invalidate() {
	s.mu.Lock()
	s.version = time.Time{}
	s.byPurpose = map[string][]oidc.ProviderConfig{}
	s.mu.Unlock()
}

func normalizeScopes(in []string) []string {
	out := []string{}
	for _, raw := range in {
		for _, field := range strings.Fields(strings.ReplaceAll(raw, ",", " ")) {
			out = append(out, field)
		}
	}
	return out
}
