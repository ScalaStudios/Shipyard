package webhooks

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Event struct {
	Provider  string
	EventType string
	Delivery  string
	Payload   json.RawMessage
}

type Service struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func VerifySignature(secret, signatureHeader string, body []byte) bool {
	if secret == "" || signatureHeader == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signatureHeader))
}

func (s *Service) Record(ctx context.Context, projectID string, event Event) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO audit_events (action, resource_type, resource_id, project_id, metadata)
		VALUES ('webhook.received', 'webhook', $1, NULLIF($2,'')::uuid, $3::jsonb)
	`, event.Delivery, projectID, fmt.Sprintf(`{"provider":%q,"event":%q,"at":%q}`, event.Provider, event.EventType, time.Now().UTC().Format(time.RFC3339)))
	return err
}
