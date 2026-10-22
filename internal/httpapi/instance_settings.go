package httpapi

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/auth"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/settings"
)

const maxAvatarBytes = 2 << 20

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

type instanceUser struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	IsAdmin     bool      `json:"is_admin"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
}

func (s *Server) handleListInstanceUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(), `
		SELECT id, username, email, display_name, is_admin, is_active, created_at
		FROM users ORDER BY created_at
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer rows.Close()
	out := []instanceUser{}
	for rows.Next() {
		var item instanceUser
		if err := rows.Scan(&item.ID, &item.Username, &item.Email, &item.DisplayName, &item.IsAdmin, &item.IsActive, &item.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		out = append(out, item)
	}
	if rows.Err() != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": out})
}

func (s *Server) handleGetInstanceUser(w http.ResponseWriter, r *http.Request) {
	var item instanceUser
	err := s.pool.QueryRow(r.Context(), `
		SELECT id, username, email, display_name, is_admin, is_active, created_at
		FROM users WHERE id::text = $1
	`, r.PathValue("userID")).Scan(&item.ID, &item.Username, &item.Email, &item.DisplayName, &item.IsAdmin, &item.IsActive, &item.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": item})
}

type updateUserRequest struct {
	Username    string `json:"username"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

func (s *Server) handleUpdateInstanceUser(w http.ResponseWriter, r *http.Request) {
	var req updateUserRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	username := strings.TrimSpace(req.Username)
	email := strings.TrimSpace(strings.ToLower(req.Email))
	displayName := strings.TrimSpace(req.DisplayName)
	if username == "" || email == "" {
		writeError(w, http.StatusBadRequest, "username and email are required")
		return
	}
	if n := utf8.RuneCountInString(username); n < 2 || n > 64 {
		writeError(w, http.StatusBadRequest, "username must be between 2 and 64 characters")
		return
	}
	if n := utf8.RuneCountInString(email); n < 3 || n > 320 {
		writeError(w, http.StatusBadRequest, "email must be between 3 and 320 characters")
		return
	}
	if displayName == "" {
		displayName = username
	}
	tag, err := s.pool.Exec(r.Context(), `
		UPDATE users SET username = $2, email = $3, display_name = $4, updated_at = now()
		WHERE id::text = $1
	`, r.PathValue("userID"), username, email, displayName)
	if err != nil {
		if strings.Contains(err.Error(), "SQLSTATE 23505") {
			writeError(w, http.StatusConflict, "username or email already in use")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "saved"})
}

type userPasswordRequest struct {
	Password string `json:"password"`
}

func (s *Server) handleSetUserPassword(w http.ResponseWriter, r *http.Request) {
	var req userPasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	target := r.PathValue("userID")
	tag, err := s.pool.Exec(r.Context(), `
		UPDATE users SET password_hash = $2, updated_at = now() WHERE id::text = $1
	`, target, hash)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if _, err := s.pool.Exec(r.Context(), `
		UPDATE sessions SET revoked_at = now()
		WHERE user_id::text = $1 AND revoked_at IS NULL
	`, target); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "saved"})
}

func (s *Server) handleSetUserAvatar(w http.ResponseWriter, r *http.Request) {
	s.storeAvatar(w, r, r.PathValue("userID"))
}

func (s *Server) handleSetMyAvatar(w http.ResponseWriter, r *http.Request) {
	s.storeAvatar(w, r, currentUser(r).ID)
}

func (s *Server) handleDeleteUserAvatar(w http.ResponseWriter, r *http.Request) {
	s.saveAvatarDigest(w, r, r.PathValue("userID"), "")
}

func (s *Server) handleDeleteMyAvatar(w http.ResponseWriter, r *http.Request) {
	s.saveAvatarDigest(w, r, currentUser(r).ID, "")
}

func (s *Server) storeAvatar(w http.ResponseWriter, r *http.Request, userID string) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "image/") {
		writeError(w, http.StatusUnsupportedMediaType, "avatar must be an image")
		return
	}
	body, err := readBodyLimited(r, maxAvatarBytes+1)
	if err != nil {
		writeError(w, http.StatusBadRequest, "could not read image")
		return
	}
	if len(body) == 0 {
		writeError(w, http.StatusBadRequest, "image is empty")
		return
	}
	if len(body) > maxAvatarBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "image must be 2 MB or smaller")
		return
	}
	info, err := s.store.Put(r.Context(), "", bytes.NewReader(body), int64(len(body)))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.saveAvatarDigest(w, r, userID, info.Digest)
}

func (s *Server) saveAvatarDigest(w http.ResponseWriter, r *http.Request, userID, digest string) {
	tag, err := s.pool.Exec(r.Context(), `
		UPDATE users SET avatar_digest = $2, updated_at = now() WHERE id::text = $1
	`, userID, digest)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	status := "saved"
	if digest == "" {
		status = "deleted"
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": status})
}

func (s *Server) handleGetUserAvatar(w http.ResponseWriter, r *http.Request) {
	var digest string
	err := s.pool.QueryRow(r.Context(), `
		SELECT avatar_digest FROM users WHERE id::text = $1
	`, r.PathValue("userID")).Scan(&digest)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if digest == "" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	rc, info, err := s.store.Get(r.Context(), digest)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	defer rc.Close()
	head := make([]byte, 512)
	n, readErr := io.ReadFull(rc, head)
	if n == 0 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	w.Header().Set("Content-Type", http.DetectContentType(head[:n]))
	w.Header().Set("Cache-Control", "private, max-age=300")
	if info.Size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
	}
	if _, err := w.Write(head[:n]); err != nil {
		return
	}
	if readErr == nil {
		_, _ = io.Copy(w, rc)
	}
}

type activeRequest struct {
	IsActive bool `json:"is_active"`
}

func (s *Server) handleSetUserActive(w http.ResponseWriter, r *http.Request) {
	var req activeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	target := r.PathValue("userID")
	if !req.IsActive && target == currentUser(r).ID {
		writeError(w, http.StatusBadRequest, "you cannot deactivate your own account")
		return
	}
	tag, err := s.pool.Exec(r.Context(), `UPDATE users SET is_active = $2 WHERE id::text = $1`, target, req.IsActive)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if !req.IsActive {
		if _, err := s.pool.Exec(r.Context(), `
			UPDATE sessions SET revoked_at = now()
			WHERE user_id::text = $1 AND revoked_at IS NULL
		`, target); err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "saved", "is_active": req.IsActive})
}

type createUserRequest struct {
	Username    string `json:"username"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
	IsAdmin     bool   `json:"is_admin"`
}

func (s *Server) handleCreateInstanceUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	user, err := s.identity.CreateUser(r.Context(), req.Username, req.Email, req.DisplayName, req.Password)
	if err != nil {
		mapIdentityError(w, err)
		return
	}
	if req.IsAdmin && !user.IsAdmin {
		if _, err := s.pool.Exec(r.Context(), `UPDATE users SET is_admin = $2 WHERE id::text = $1`, user.ID, true); err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		user.IsAdmin = true
	}
	writeJSON(w, http.StatusCreated, map[string]any{"user": user})
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
	tag, err := s.pool.Exec(r.Context(), `UPDATE users SET is_admin = $2 WHERE id::text = $1`, target, req.IsAdmin)
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
	Enabled      *bool    `json:"enabled"`
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
	}, req.Enabled, currentUser(r).ID)
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
