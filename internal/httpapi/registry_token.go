package httpapi

import (
	"net/http"
	"time"
)

func (s *Server) handleRegistryToken(w http.ResponseWriter, r *http.Request) {
	user, err := s.authenticate(r)
	if err != nil {
		w.Header().Set("WWW-Authenticate", `Basic realm="shipyard-registry"`)
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	token := bearerToken(r)
	if token == "" {
		if _, pass, ok := basicAuth(r); ok {
			token = pass
		}
	}
	if token == "" {
		writeError(w, http.StatusBadRequest, "provide an API token via Basic password or Bearer")
		return
	}
	_ = user
	writeJSON(w, http.StatusOK, map[string]any{
		"token":        token,
		"access_token": token,
		"expires_in":   int((12 * time.Hour).Seconds()),
		"issued_at":    time.Now().UTC().Format(time.RFC3339),
	})
}
