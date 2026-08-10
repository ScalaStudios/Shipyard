package httpapi

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/githubapp"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/rbac"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/scm"
)

const (
	keyGitHubAppID      = "github_app.app_id"
	keyGitHubAppSlug    = "github_app.slug"
	keyGitHubAppKey     = "github_app.private_key"
	keyGitHubAppAPIBase = "github_app.api_base_url"
)

func (s *Server) githubAppConfig(ctx context.Context) githubapp.Config {
	values := s.settings.Values(ctx)
	return githubapp.Config{
		AppID:         values[keyGitHubAppID],
		Slug:          values[keyGitHubAppSlug],
		PrivateKeyPEM: values[keyGitHubAppKey],
		APIBaseURL:    values[keyGitHubAppAPIBase],
	}
}

func (s *Server) installationToken(ctx context.Context, installationID string) (string, error) {
	cfg := s.githubAppConfig(ctx)
	if !cfg.Ready() {
		return "", githubAppNotConfigured
	}
	token, _, err := githubapp.InstallationToken(ctx, cfg, installationID)
	return token, err
}

var githubAppNotConfigured = errNotConfigured("github app is not configured: add the app id and private key in settings")

type errNotConfigured string

func (e errNotConfigured) Error() string { return string(e) }

func (s *Server) handleGitHubAppStatus(w http.ResponseWriter, r *http.Request) {
	cfg := s.githubAppConfig(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"configured":   cfg.Ready(),
		"slug":         cfg.Slug,
		"app_id":       cfg.AppID,
		"has_key":      strings.TrimSpace(cfg.PrivateKeyPEM) != "",
		"callback_url": s.apiURL(r.Context()) + "/api/v1/forge/github-app/callback",
	})
}

func (s *Server) handleGitHubAppInstallations(w http.ResponseWriter, r *http.Request) {
	cfg := s.githubAppConfig(r.Context())
	if !cfg.Ready() {
		writeError(w, http.StatusServiceUnavailable, githubAppNotConfigured.Error())
		return
	}
	list, err := githubapp.ListInstallations(r.Context(), cfg)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"installations": list})
}

func (s *Server) handleGitHubAppInstall(w http.ResponseWriter, r *http.Request) {
	org, ok := s.orgAccess(w, r, rbac.PermOrgUpdate)
	if !ok {
		return
	}
	cfg := s.githubAppConfig(r.Context())
	if !cfg.Ready() || cfg.Slug == "" {
		writeError(w, http.StatusServiceUnavailable, "github app is not configured: add the app id, slug and private key in settings")
		return
	}
	state, err := s.forgeOAuth.IssueState(org.ID + "|" + currentUser(r).ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	target := githubapp.InstallURL(cfg, state)
	if r.URL.Query().Get("redirect") == "false" {
		writeJSON(w, http.StatusOK, map[string]any{"install_url": target})
		return
	}
	http.Redirect(w, r, target, http.StatusFound)
}

type linkInstallRequest struct {
	InstallationID string `json:"installation_id"`
}

func (s *Server) handleGitHubAppLink(w http.ResponseWriter, r *http.Request) {
	org, ok := s.orgAccess(w, r, rbac.PermOrgUpdate)
	if !ok {
		return
	}
	var req linkInstallRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	installationID := strings.TrimSpace(req.InstallationID)
	if _, err := strconv.ParseInt(installationID, 10, 64); err != nil {
		writeError(w, http.StatusBadRequest, "installation_id must be numeric")
		return
	}
	cfg := s.githubAppConfig(r.Context())
	if _, _, err := githubapp.InstallationToken(r.Context(), cfg, installationID); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	cred, err := s.scm.UpsertCredential(r.Context(), scm.CreateCredentialInput{
		OrganizationID: org.ID,
		Provider:       "github",
		Kind:           scm.KindGitHubApp,
		Name:           "github-app-" + installationID,
		BaseURL:        "https://github.com",
		InstallationID: installationID,
		ActorID:        currentUser(r).ID,
	})
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"credential": cred})
}

func (s *Server) redirectPendingInstall(w http.ResponseWriter, r *http.Request, installationID string) {
	target := s.publicURL(r.Context()) + "/projects/import?github_installation=" + url.QueryEscape(installationID)
	http.Redirect(w, r, target, http.StatusFound)
}

func (s *Server) handleGitHubAppCallback(w http.ResponseWriter, r *http.Request) {
	installationID := strings.TrimSpace(r.URL.Query().Get("installation_id"))
	state := r.URL.Query().Get("state")
	if installationID == "" {
		s.redirectImport(w, r, "", "github did not return an installation id")
		return
	}
	if _, err := strconv.ParseInt(installationID, 10, 64); err != nil {
		s.redirectImport(w, r, "", "github returned an invalid installation id")
		return
	}
	data, ok := s.forgeOAuth.ConsumeState(state)
	if !ok {
		s.redirectPendingInstall(w, r, installationID)
		return
	}
	orgID, userID, _ := strings.Cut(data, "|")
	if orgID == "" || userID == "" || userID != currentUser(r).ID {
		s.redirectImport(w, r, "", "install state does not match the signed-in user")
		return
	}
	if _, _, err := s.orgs.Require(r.Context(), userID, orgID, rbac.PermOrgUpdate); err != nil {
		s.redirectImport(w, r, "", "not allowed to add credentials to this organization")
		return
	}

	cfg := s.githubAppConfig(r.Context())
	if _, _, err := githubapp.InstallationToken(r.Context(), cfg, installationID); err != nil {
		s.redirectImport(w, r, "", err.Error())
		return
	}

	cred, err := s.scm.UpsertCredential(r.Context(), scm.CreateCredentialInput{
		OrganizationID: orgID,
		Provider:       "github",
		Kind:           scm.KindGitHubApp,
		Name:           "github-app-" + installationID,
		BaseURL:        "https://github.com",
		InstallationID: installationID,
		ActorID:        userID,
	})
	if err != nil {
		s.redirectImport(w, r, "", err.Error())
		return
	}
	s.redirectImport(w, r, cred.ID, "")
}
