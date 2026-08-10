package httpapi

import (
	"net/http"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/settings"
)

func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		if !currentUser(r).IsAdmin {
			writeError(w, http.StatusForbidden, "instance admin required")
			return
		}
		next(w, r)
	})
}

func (s *Server) handleListInstanceAdmins(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(), `
		SELECT id, username, email, is_admin FROM users WHERE is_active ORDER BY is_admin DESC, username
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()
	type row struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Email    string `json:"email"`
		IsAdmin  bool   `json:"is_admin"`
	}
	out := []row{}
	for rows.Next() {
		var item row
		if err := rows.Scan(&item.ID, &item.Username, &item.Email, &item.IsAdmin); err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		out = append(out, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": out})
}

type adminRequest struct {
	IsAdmin bool `json:"is_admin"`
}

func (s *Server) handleSetInstanceAdmin(w http.ResponseWriter, r *http.Request) {
	var req adminRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	target := r.PathValue("userID")
	if !req.IsAdmin && target == currentUser(r).ID {
		writeError(w, http.StatusBadRequest, "you cannot remove your own admin access")
		return
	}
	tag, err := s.pool.Exec(r.Context(), `UPDATE users SET is_admin = $2 WHERE id = $1`, target, req.IsAdmin)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "saved", "is_admin": req.IsAdmin})
}

func (s *Server) handleListInstanceSettings(w http.ResponseWriter, r *http.Request) {
	list, err := s.settings.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": list})
}

type settingRequest struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	IsSecret bool   `json:"is_secret"`
}

func (s *Server) handleSetInstanceSetting(w http.ResponseWriter, r *http.Request) {
	var req settingRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := s.settings.Set(r.Context(), req.Key, req.Value, req.IsSecret, currentUser(r).ID); err != nil {
		mapIdentityError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "saved"})
}

func (s *Server) handleListAuthProviders(w http.ResponseWriter, r *http.Request) {
	purpose := r.URL.Query().Get("purpose")
	if purpose == "" {
		purpose = settings.PurposeLogin
	}
	list, err := s.settings.ListProviders(r.Context(), purpose)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": list, "callback_base": s.apiURL(r.Context())})
}

type authProviderRequest struct {
	Purpose      string   `json:"purpose"`
	Name         string   `json:"name"`
	Kind         string   `json:"kind"`
	Issuer       string   `json:"issuer"`
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	RedirectURL  string   `json:"redirect_url"`
	Scopes       []string `json:"scopes"`
	Enabled      bool     `json:"enabled"`
}

func (s *Server) handleUpsertAuthProvider(w http.ResponseWriter, r *http.Request) {
	var req authProviderRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	provider, err := s.settings.UpsertProvider(r.Context(), settings.Provider{
		Purpose:      req.Purpose,
		Name:         req.Name,
		Kind:         req.Kind,
		Issuer:       req.Issuer,
		ClientID:     req.ClientID,
		ClientSecret: req.ClientSecret,
		RedirectURL:  req.RedirectURL,
		Scopes:       req.Scopes,
		Enabled:      req.Enabled,
	}, currentUser(r).ID)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	s.reloadAuthProviders(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"provider": provider})
}

func (s *Server) handleDeleteAuthProvider(w http.ResponseWriter, r *http.Request) {
	purpose := r.URL.Query().Get("purpose")
	if purpose == "" {
		purpose = settings.PurposeLogin
	}
	if err := s.settings.DeleteProvider(r.Context(), purpose, r.PathValue("providerID")); err != nil {
		mapIdentityError(w, err)
		return
	}
	s.reloadAuthProviders(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
}
