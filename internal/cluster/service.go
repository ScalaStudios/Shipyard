package cluster

import (
	"context"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool   *pgxpool.Pool
	nodeID string
}

func New(pool *pgxpool.Pool, nodeID string) *Service {
	if nodeID == "" {
		host, _ := os.Hostname()
		nodeID = host
	}
	return &Service{pool: pool, nodeID: nodeID}
}

func (s *Service) Heartbeat(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO cluster_nodes (node_id, last_seen_at)
		VALUES ($1, now())
		ON CONFLICT (node_id) DO UPDATE SET last_seen_at = now()
	`, s.nodeID)
	return err
}

func (s *Service) AcquireLease(ctx context.Context, name string, ttl time.Duration) (bool, int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, 0, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, name); err != nil {
		return false, 0, err
	}

	var owner string
	var token int64
	var expires time.Time
	err = tx.QueryRow(ctx, `
		SELECT owner_node_id, fencing_token, expires_at FROM cluster_leases WHERE name = $1
	`, name).Scan(&owner, &token, &expires)
	now := time.Now().UTC()
	if err == nil && owner != s.nodeID && expires.After(now) {
		return false, 0, tx.Commit(ctx)
	}

	next := token + 1
	if err != nil {
		next = 1
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO cluster_leases (name, owner_node_id, fencing_token, expires_at)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (name) DO UPDATE
		SET owner_node_id = EXCLUDED.owner_node_id,
		    fencing_token = EXCLUDED.fencing_token,
		    expires_at = EXCLUDED.expires_at
	`, name, s.nodeID, next, now.Add(ttl))
	if err != nil {
		return false, 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, 0, err
	}
	return true, next, nil
}

func (s *Service) NodeID() string { return s.nodeID }
