package httpapi

import (
	"net/http"
	"strings"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/rbac"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/secrets"
)

type secretRequest struct {
	Name           string `json:"name"`
	Value          string `json:"value"`
	OrganizationID string `json:"organization_id"`
	ProjectID      string `json:"project_id"`
	EnvironmentID  string `json:"environment_id"`
}

func (s *Server) handleCreateSecret(w http.ResponseWriter, r *http.Request) {
	if s.secrets == nil {
		writeError(w, http.StatusServiceUnavailable, "secrets not configured")
		return
	}
	var req secretRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.OrganizationID == "" {
		writeError(w, http.StatusBadRequest, "organization_id required")
		return
	}
	org, _, err := s.orgs.Require(r.Context(), currentUser(r).ID, req.OrganizationID, rbac.PermOrgUpdate)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	if req.ProjectID != "" {
		if _, err := s.orgs.GetProject(r.Context(), org.ID, req.ProjectID); err != nil {
			mapIdentityError(w, err)
			return
		}
	}
	meta, err := s.secrets.Put(r.Context(), req.OrganizationID, req.ProjectID, req.EnvironmentID, currentUser(r).ID, req.Name, req.Value)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"secret": meta})
}

func (s *Server) handleListSecrets(w http.ResponseWriter, r *http.Request) {
	if s.secrets == nil {
		writeError(w, http.StatusServiceUnavailable, "secrets not configured")
		return
	}
	orgID := r.URL.Query().Get("organization_id")
	projectID := r.URL.Query().Get("project_id")
	if orgID == "" {
		writeError(w, http.StatusBadRequest, "organization_id required")
		return
	}
	if _, _, err := s.orgs.Require(r.Context(), currentUser(r).ID, orgID, rbac.PermOrgRead); err != nil {
		mapIdentityError(w, err)
		return
	}
	rows, err := s.pool.Query(r.Context(), `
		SELECT id, name,
			CASE
				WHEN environment_id IS NOT NULL THEN 'environment'
				WHEN project_id IS NOT NULL THEN 'project'
				ELSE 'organization'
			END AS scope
		FROM secrets
		WHERE ($1 = '' OR organization_id::text = $1)
		  AND ($2 = '' OR project_id::text = $2)
		ORDER BY name
		LIMIT 200
	`, orgID, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()
	var out []secrets.SecretMeta
	for rows.Next() {
		var m secrets.SecretMeta
		if err := rows.Scan(&m.ID, &m.Name, &m.Scope); err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		out = append(out, m)
	}
	if out == nil {
		out = []secrets.SecretMeta{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"secrets": out})
}

func (s *Server) handleListOIDCProviders(w http.ResponseWriter, _ *http.Request) {
	if s.oidc == nil || !s.oidc.Enabled() {
		writeJSON(w, http.StatusOK, map[string]any{"providers": []any{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": s.oidc.List()})
}

func (s *Server) handleOIDCStart(w http.ResponseWriter, r *http.Request) {
	if s.oidc == nil || !s.oidc.Enabled() {
		writeError(w, http.StatusNotFound, "oidc not configured")
		return
	}
	provider := r.PathValue("provider")
	authURL, _, err := s.oidc.AuthURL(provider)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (s *Server) handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	if s.oidc == nil || !s.oidc.Enabled() {
		writeError(w, http.StatusNotFound, "oidc not configured")
		return
	}
	provider := r.PathValue("provider")
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	tok, err := s.oidc.Exchange(r.Context(), provider, code, state)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	email := strings.TrimSpace(tok.Email)
	if email == "" {
		writeError(w, http.StatusBadRequest, "oidc token missing email")
		return
	}
	username := strings.TrimSpace(tok.Username)
	if username == "" {
		username = strings.Split(email, "@")[0]
	}
	username = provider + "-" + username
	username = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, username)
	if len(username) > 64 {
		username = username[:64]
	}
	user, err := s.identity.EnsureOIDCUser(r.Context(), provider, tok.Subject, username, email, tok.Name, tok.EmailVerified)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	session, expires, err := s.identity.CreateSession(r.Context(), user.ID, s.opts.SessionTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.setSessionCookie(w, r, session, expires)
	http.Redirect(w, r, s.publicURL(r.Context())+"/", http.StatusFound)
}
