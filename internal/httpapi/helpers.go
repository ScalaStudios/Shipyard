package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/auth"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/identity"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/rbac"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/runners"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/secrets"
)

const sessionCookie = "shipyard_session"

type ctxKey int

const (
	userKey   ctxKey = 1
	runnerKey ctxKey = 2
	scopesKey ctxKey = 3
)

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, scopes, err := s.authenticate(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if len(scopes) > 0 {
			writeError(w, http.StatusForbidden, "this token is scoped to the package registry")
			return
		}
		ctx := context.WithValue(r.Context(), userKey, user)
		ctx = context.WithValue(ctx, scopesKey, scopes)
		next(w, r.WithContext(ctx))
	}
}

func (s *Server) requireRegistryAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, scopes, err := s.authenticate(r)
		if err != nil {
			base := s.apiBaseURL(r)
			w.Header().Set("WWW-Authenticate", `Bearer realm="`+base+`/auth/token",service="shipyard"`)
			w.Header().Add("WWW-Authenticate", `Basic realm="shipyard-registry"`)
			writeOCIError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
			return
		}
		ctx := context.WithValue(r.Context(), userKey, user)
		ctx = context.WithValue(ctx, scopesKey, scopes)
		next(w, r.WithContext(ctx))
	}
}

func (s *Server) requireRunner(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		runner, err := s.runners.RunnerFromToken(r.Context(), token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), runnerKey, runner)))
	}
}

func currentUser(r *http.Request) identity.User {
	u, _ := r.Context().Value(userKey).(identity.User)
	return u
}

func currentScopes(r *http.Request) []string {
	s, _ := r.Context().Value(scopesKey).([]string)
	return s
}

func registryScopeOK(r *http.Request, perm rbac.Permission) bool {
	required := identity.ScopeRegistryRead
	if perm != rbac.PermProjectRead {
		required = identity.ScopeRegistryWrite
	}
	return identity.ScopeAllows(currentScopes(r), required)
}

func currentRunner(r *http.Request) runners.Runner {
	runner, _ := r.Context().Value(runnerKey).(runners.Runner)
	return runner
}

func (s *Server) authenticate(r *http.Request) (identity.User, []string, error) {
	if token := bearerToken(r); token != "" {
		if user, scopes, err := s.identity.UserFromAPIToken(r.Context(), token); err == nil {
			return user, scopes, nil
		}
	}
	if user, pass, ok := basicAuth(r); ok {
		if pass != "" {
			if u, scopes, err := s.identity.UserFromAPIToken(r.Context(), pass); err == nil {
				return u, scopes, nil
			}
		}
		if user != "" && pass != "" {
			if u, err := s.identity.Authenticate(r.Context(), user, pass); err == nil {
				return u, nil, nil
			}
		}
	}
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		return identity.User{}, nil, identity.ErrUnauthorized
	}
	u, err := s.identity.UserFromSession(r.Context(), c.Value)
	return u, nil, err
}

func bearerToken(r *http.Request) string {
	authz := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
		return strings.TrimSpace(authz[7:])
	}
	return ""
}

func basicAuth(r *http.Request) (username, password string, ok bool) {
	authz := r.Header.Get("Authorization")
	if !strings.HasPrefix(strings.ToLower(authz), "basic ") {
		return "", "", false
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(authz[6:]))
	if err != nil {
		return "", "", false
	}
	parts := strings.SplitN(string(raw), ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func (s *Server) requestIsHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		return true
	}
	return strings.HasPrefix(s.publicURL(r.Context()), "https://")
}

func (s *Server) setSessionCookie(w http.ResponseWriter, r *http.Request, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   s.requestIsHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "http://127.0.0.1:5173" || origin == "http://localhost:5173" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}

func mapIdentityError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, identity.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, "unauthorized")
	case errors.Is(err, identity.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, identity.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, identity.ErrConflict):
		writeError(w, http.StatusConflict, "conflict")
	case errors.Is(err, identity.ErrInvalidInput), errors.Is(err, auth.ErrInvalidPassword):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, secrets.ErrNotConfigured):
		writeError(w, http.StatusServiceUnavailable, err.Error())
	default:
		if strings.Contains(err.Error(), "parse shipyard.yml") || strings.Contains(err.Error(), "pipeline must") {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
