package settings

import (
	"context"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/oidc"
)

func (s *Service) SeedProvidersFromEnv(ctx context.Context, login, forge []oidc.ProviderConfig) (int, error) {
	var count int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM auth_providers`).Scan(&count); err != nil {
		return 0, err
	}
	if count > 0 {
		return 0, nil
	}

	seeded := 0
	for _, group := range []struct {
		purpose string
		configs []oidc.ProviderConfig
	}{{PurposeLogin, login}, {PurposeForge, forge}} {
		for _, cfg := range group.configs {
			cfg = oidc.Normalize(cfg)
			if _, err := s.UpsertProvider(ctx, Provider{
				Purpose:      group.purpose,
				Name:         cfg.Name,
				Kind:         string(cfg.Kind),
				Issuer:       cfg.Issuer,
				ClientID:     cfg.ClientID,
				ClientSecret: cfg.ClientSecret,
				RedirectURL:  cfg.RedirectURL,
				Scopes:       cfg.Scopes,
				Enabled:      true,
			}, nil, ""); err != nil {
				return seeded, err
			}
			seeded++
		}
	}
	return seeded, nil
}
