package audit

import (
	"context"
	"encoding/json"
	"net"
	"net/netip"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Event struct {
	ActorUserID    *string
	Action         string
	ResourceType   string
	ResourceID     string
	OrganizationID *string
	ProjectID      *string
	IP             string
	UserAgent      string
	Metadata       map[string]any
}

type Logger struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Logger {
	return &Logger{pool: pool}
}

func (l *Logger) Record(ctx context.Context, event Event) error {
	meta := event.Metadata
	if meta == nil {
		meta = map[string]any{}
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		return err
	}

	var ip any
	if event.IP != "" {
		if addr, err := netip.ParseAddr(event.IP); err == nil {
			ip = addr.String()
		} else if host, _, err := net.SplitHostPort(event.IP); err == nil {
			if addr, err := netip.ParseAddr(host); err == nil {
				ip = addr.String()
			}
		}
	}

	_, err = l.pool.Exec(ctx, `
		INSERT INTO audit_events (
			actor_user_id, action, resource_type, resource_id,
			organization_id, project_id, ip, user_agent, metadata
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb)
	`, event.ActorUserID, event.Action, event.ResourceType, event.ResourceID,
		event.OrganizationID, event.ProjectID, ip, event.UserAgent, string(raw))
	return err
}
