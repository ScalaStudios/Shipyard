package notifications

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Notification struct {
	ID             string     `json:"id"`
	UserID         string     `json:"user_id,omitempty"`
	OrganizationID string     `json:"organization_id,omitempty"`
	ProjectID      string     `json:"project_id,omitempty"`
	Kind           string     `json:"kind"`
	Title          string     `json:"title"`
	Body           string     `json:"body"`
	Href           string     `json:"href"`
	ReadAt         *time.Time `json:"read_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type Service struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

type CreateInput struct {
	UserID         string
	OrganizationID string
	ProjectID      string
	Kind           string
	Title          string
	Body           string
	Href           string
}

func (s *Service) Create(ctx context.Context, in CreateInput) (Notification, error) {
	var n Notification
	err := s.pool.QueryRow(ctx, `
		INSERT INTO notifications (user_id, organization_id, project_id, kind, title, body, href)
		VALUES (NULLIF($1,'')::uuid, NULLIF($2,'')::uuid, NULLIF($3,'')::uuid, $4, $5, $6, $7)
		RETURNING id, COALESCE(user_id::text,''), COALESCE(organization_id::text,''), COALESCE(project_id::text,''),
		          kind, title, body, href, read_at, created_at
	`, in.UserID, in.OrganizationID, in.ProjectID, in.Kind, in.Title, in.Body, in.Href).
		Scan(&n.ID, &n.UserID, &n.OrganizationID, &n.ProjectID, &n.Kind, &n.Title, &n.Body, &n.Href, &n.ReadAt, &n.CreatedAt)
	return n, err
}

func (s *Service) NotifyOrgMembers(ctx context.Context, orgID, projectID, kind, title, body, href string) error {
	rows, err := s.pool.Query(ctx, `
		SELECT user_id FROM organization_members WHERE organization_id = $1
	`, orgID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return err
		}
		if _, err := s.Create(ctx, CreateInput{
			UserID: userID, OrganizationID: orgID, ProjectID: projectID,
			Kind: kind, Title: title, Body: body, Href: href,
		}); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (s *Service) ListForUser(ctx context.Context, userID string, unreadOnly bool, limit int) ([]Notification, error) {
	if limit <= 0 || limit > 100 {
		limit = 40
	}
	q := `
		SELECT id, COALESCE(user_id::text,''), COALESCE(organization_id::text,''), COALESCE(project_id::text,''),
		       kind, title, body, href, read_at, created_at
		FROM notifications WHERE user_id = $1
	`
	if unreadOnly {
		q += ` AND read_at IS NULL`
	}
	q += ` ORDER BY created_at DESC LIMIT $2`
	rows, err := s.pool.Query(ctx, q, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Notification
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.UserID, &n.OrganizationID, &n.ProjectID, &n.Kind, &n.Title, &n.Body, &n.Href, &n.ReadAt, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Service) UnreadCount(ctx context.Context, userID string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND read_at IS NULL`, userID).Scan(&n)
	return n, err
}

func (s *Service) MarkRead(ctx context.Context, userID, id string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE notifications SET read_at = now() WHERE user_id = $1 AND id = $2 AND read_at IS NULL
	`, userID, id)
	return err
}

func (s *Service) MarkAllRead(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE notifications SET read_at = now() WHERE user_id = $1 AND read_at IS NULL
	`, userID)
	return err
}
