package httpapi

import (
	"net/http"
	"net/url"
	"strings"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/rbac"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/scm"
)

func (s *Server) handleListForgeOAuthProviders(w http.ResponseWriter, r *http.Request) {
	list := []map[string]string{}
	if s.forgeOAuth != nil {
		for _, p := range s.forgeOAuth.List() {
			list = append(list, map[string]string{"name": p.Name, "kind": p.Kind})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": list})
}

func (s *Server) handleForgeOAuthStart(w http.ResponseWriter, r *http.Request) {
	org, ok := s.orgAccess(w, r, rbac.PermOrgUpdate)
	if !ok {
		return
	}
	if s.forgeOAuth == nil {
		writeError(w, http.StatusServiceUnavailable, "forge oauth not configured")
		return
	}
	provider := scm.NormalizeProvider(r.PathValue("provider"))
	authURL, _, err := s.forgeOAuth.AuthURLFor(provider, org.ID+"|"+currentUser(r).ID)
	if err != nil {
		writeError(w, http.StatusNotFound, "unknown forge oauth provider")
		return
	}
	if r.URL.Query().Get("redirect") == "false" {
		writeJSON(w, http.StatusOK, map[string]any{"authorize_url": authURL})
		return
	}
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (s *Server) handleForgeOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if s.forgeOAuth == nil {
		writeError(w, http.StatusServiceUnavailable, "forge oauth not configured")
		return
	}
	provider := scm.NormalizeProvider(r.PathValue("provider"))
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		s.redirectImport(w, r, "", "missing code or state")
		return
	}
	result, err := s.forgeOAuth.Exchange(r.Context(), provider, code, state)
	if err != nil {
		s.redirectImport(w, r, "", err.Error())
		return
	}
	orgID, userID, _ := strings.Cut(result.StateData, "|")
	if orgID == "" || userID == "" || userID != currentUser(r).ID {
		s.redirectImport(w, r, "", "oauth state does not match the signed-in user")
		return
	}
	if _, _, err := s.orgs.Require(r.Context(), userID, orgID, rbac.PermOrgUpdate); err != nil {
		s.redirectImport(w, r, "", "not allowed to add credentials to this organization")
		return
	}
	cred, err := s.scm.UpsertCredential(r.Context(), scm.CreateCredentialInput{
		OrganizationID: orgID,
		Provider:       provider,
		Kind:           "oauth_user",
		Name:           forgeCredentialName(provider, result.Username, result.Email),
		BaseURL:        s.forgeOAuth.Issuer(provider),
		AccessToken:    result.AccessToken,
		ActorID:        userID,
	})
	if err != nil {
		s.redirectImport(w, r, "", err.Error())
		return
	}
	s.redirectImport(w, r, cred.ID, "")
}

func (s *Server) redirectImport(w http.ResponseWriter, r *http.Request, credentialID, errMsg string) {
	target := s.publicURL(r.Context()) + "/projects/import"
	q := url.Values{}
	if credentialID != "" {
		q.Set("credential_id", credentialID)
	}
	if errMsg != "" {
		q.Set("error", errMsg)
	}
	if len(q) > 0 {
		target += "?" + q.Encode()
	}
	http.Redirect(w, r, target, http.StatusFound)
}

func forgeCredentialName(provider, username, email string) string {
	raw := username
	if raw == "" {
		raw, _, _ = strings.Cut(email, "@")
	}
	var b strings.Builder
	for _, c := range strings.ToLower(raw) {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			b.WriteRune(c)
		} else {
			b.WriteByte('-')
		}
	}
	name := strings.Trim(b.String(), "-")
	if name == "" {
		name = "user"
	}
	if len(name) > 40 {
		name = strings.Trim(name[:40], "-")
	}
	return provider + "-" + name
}
