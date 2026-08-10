package httpapi

import (
	"encoding/json"
	"io"
	"net/http"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/webhooks"
)

func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	provider := r.PathValue("provider")
	secret := r.Header.Get("X-Shipyard-Webhook-Secret")
	sig := r.Header.Get("X-Hub-Signature-256")
	if secret != "" && sig != "" && !webhooks.VerifySignature(secret, sig, body) {
		writeError(w, http.StatusUnauthorized, "invalid signature")
		return
	}
	eventType := r.Header.Get("X-GitHub-Event")
	if eventType == "" {
		eventType = r.Header.Get("X-Gitea-Event")
	}
	if eventType == "" {
		eventType = r.Header.Get("X-Forgejo-Event")
	}
	if eventType == "" {
		eventType = "unknown"
	}
	delivery := r.Header.Get("X-GitHub-Delivery")
	if delivery == "" {
		delivery = r.Header.Get("X-Gitea-Delivery")
	}
	svc := webhooks.New(s.pool)
	if err := svc.Record(r.Context(), r.URL.Query().Get("project_id"), webhooks.Event{
		Provider:  provider,
		EventType: eventType,
		Delivery:  delivery,
		Payload:   json.RawMessage(body),
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "accepted"})
}
