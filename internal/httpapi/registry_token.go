package httpapi

import (
	"net/http"
	"time"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/identity"
)

func (s *Server) handleRegistryToken(w http.ResponseWriter, r *http.Request) {
	user, _, err := s.authenticate(r)
	if err != nil {
		w.Header().Set("WWW-Authenticate", `Basic realm="shipyard-registry"`)
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	cred := bearerToken(r)
	if cred == "" {
		if _, pass, ok := basicAuth(r); ok {
			cred = pass
		}
	}
	token := ""
	if cred != "" {
		if _, _, err := s.identity.UserFromAPIToken(r.Context(), cred); err == nil {
			token = cred
		}
	}
	if token == "" {
		ttl := 12 * time.Hour
		plain, _, _, err := s.identity.CreateAPIToken(r.Context(), user.ID, "registry", &ttl, []string{identity.ScopeRegistryRead, identity.ScopeRegistryWrite})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		token = plain
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token":        token,
		"access_token": token,
		"expires_in":   43200,
		"issued_at":    time.Now().UTC().Format(time.RFC3339),
	})
}
