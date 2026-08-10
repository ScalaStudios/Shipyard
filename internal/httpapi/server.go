package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/storage"
)

type Server struct {
	pool    *pgxpool.Pool
	store   storage.Store
	started time.Time
	mux     *http.ServeMux
}

func New(pool *pgxpool.Pool, store storage.Store) *Server {
	s := &Server{
		pool:    pool,
		store:   store,
		started: time.Now().UTC(),
		mux:     http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("GET /readyz", s.handleReadyz)
	s.mux.HandleFunc("GET /api/v1/system/info", s.handleSystemInfo)
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := s.pool.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status": "not_ready",
			"error":  "database unavailable",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ready",
	})
}

func (s *Server) handleSystemInfo(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"product":    "shipyard",
		"phase":      "0-foundation",
		"started_at": s.started.Format(time.RFC3339),
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
