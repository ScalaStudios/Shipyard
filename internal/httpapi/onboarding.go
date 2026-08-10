package httpapi

import (
	"net/http"
	"time"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/settings"
)

type onboardingStep struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Detail   string `json:"detail"`
	Done     bool   `json:"done"`
	Optional bool   `json:"optional"`
	Href     string `json:"href"`
	Action   string `json:"action"`
}

func (s *Server) handleOnboarding(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := currentUser(r)

	var orgs, projects, runners, loginProviders, forgeProviders, credentials int
	_ = s.pool.QueryRow(ctx, `SELECT count(*) FROM organizations`).Scan(&orgs)
	_ = s.pool.QueryRow(ctx, `SELECT count(*) FROM projects`).Scan(&projects)
	_ = s.pool.QueryRow(ctx, `SELECT count(*) FROM runners WHERE last_heartbeat_at > now() - interval '60 seconds'`).Scan(&runners)
	_ = s.pool.QueryRow(ctx, `SELECT count(*) FROM auth_providers WHERE purpose = $1 AND enabled`, settings.PurposeLogin).Scan(&loginProviders)
	_ = s.pool.QueryRow(ctx, `SELECT count(*) FROM auth_providers WHERE purpose = $1 AND enabled`, settings.PurposeForge).Scan(&forgeProviders)
	_ = s.pool.QueryRow(ctx, `SELECT count(*) FROM forge_credentials`).Scan(&credentials)

	steps := []onboardingStep{
		{
			ID:     "encryption",
			Title:  "Enable credential encryption",
			Detail: "Set SHIPYARD_SECRETS_KEY so tokens and client secrets are encrypted at rest.",
			Done:   s.secrets != nil,
			Href:   "/settings/instance",
			Action: "How to set it",
		},
		{
			ID:     "instance_urls",
			Title:  "Confirm instance URLs",
			Detail: "Public URL and API URL are used in webhook links, OAuth redirects and runner install scripts.",
			Done:   s.publicURL(ctx) != "" && s.apiURL(ctx) != "",
			Href:   "/settings/instance",
			Action: "Edit URLs",
		},
		{
			ID:     "organization",
			Title:  "Create an organization",
			Detail: "Organizations own projects, runners and credentials.",
			Done:   orgs > 0,
			Href:   "/projects",
			Action: "Go to projects",
		},
		{
			ID:       "login_provider",
			Title:    "Add a sign-in provider",
			Detail:   "Let your team sign in with GitHub, Forgejo, Entra or any OIDC provider.",
			Done:     loginProviders > 0,
			Optional: true,
			Href:     "/settings/authentication",
			Action:   "Configure sign-in",
		},
		{
			ID:       "forge_app",
			Title:    "Connect a forge",
			Detail:   "Register a forge application, or paste a personal access token, to import repositories.",
			Done:     forgeProviders > 0 || credentials > 0,
			Optional: true,
			Href:     "/settings/authentication",
			Action:   "Configure forge",
		},
		{
			ID:     "project",
			Title:  "Import or create a project",
			Detail: "Bring repositories in from your forge, or create a project by hand.",
			Done:   projects > 0,
			Href:   "/projects/import",
			Action: "Import from forge",
		},
		{
			ID:     "runner",
			Title:  "Register a runner",
			Detail: "Runners execute pipeline jobs. At least one must be online to run anything.",
			Done:   runners > 0,
			Href:   "/runners",
			Action: "Add a runner",
		},
	}

	remaining := 0
	for _, step := range steps {
		if !step.Done && !step.Optional {
			remaining++
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"steps":      steps,
		"remaining":  remaining,
		"complete":   remaining == 0,
		"is_admin":   user.IsAdmin,
		"checked_at": time.Now().UTC(),
	})
}
