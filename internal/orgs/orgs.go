package orgs

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/identity"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/rbac"
)

type Organization struct {
	ID          string    `json:"id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	Role        string    `json:"role,omitempty"`
}

type Member struct {
	UserID      string    `json:"user_id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
}

type Project struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	Slug           string    `json:"slug"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	CreatedAt      time.Time `json:"created_at"`
}

type Service struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func (s *Service) CreateOrganization(ctx context.Context, actorID, slug, name, description string) (Organization, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if slug == "" || name == "" {
		return Organization{}, fmt.Errorf("%w: slug and name are required", identity.ErrInvalidInput)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Organization{}, err
	}
	defer tx.Rollback(ctx)

	var org Organization
	err = tx.QueryRow(ctx, `
		INSERT INTO organizations (slug, name, description, created_by)
		VALUES ($1, $2, $3, $4)
		RETURNING id, slug, name, description, created_at
	`, slug, name, description, actorID).Scan(&org.ID, &org.Slug, &org.Name, &org.Description, &org.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "SQLSTATE 23505") {
			return Organization{}, identity.ErrConflict
		}
		return Organization{}, err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO organization_members (organization_id, user_id, role)
		VALUES ($1, $2, $3)
	`, org.ID, actorID, rbac.RoleOwner); err != nil {
		return Organization{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Organization{}, err
	}
	org.Role = string(rbac.RoleOwner)
	return org, nil
}

func (s *Service) ListOrganizations(ctx context.Context, userID string) ([]Organization, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT o.id, o.slug, o.name, o.description, o.created_at, m.role
		FROM organizations o
		JOIN organization_members m ON m.organization_id = o.id
		WHERE m.user_id = $1
		ORDER BY o.slug
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Organization
	for rows.Next() {
		var org Organization
		if err := rows.Scan(&org.ID, &org.Slug, &org.Name, &org.Description, &org.CreatedAt, &org.Role); err != nil {
			return nil, err
		}
		out = append(out, org)
	}
	return out, rows.Err()
}

func (s *Service) GetOrganization(ctx context.Context, userID, orgID string) (Organization, rbac.Role, error) {
	orgID = strings.ToLower(strings.TrimSpace(orgID))
	var org Organization
	var role string
	err := s.pool.QueryRow(ctx, `
		SELECT o.id, o.slug, o.name, o.description, o.created_at, m.role
		FROM organizations o
		JOIN organization_members m ON m.organization_id = o.id
		WHERE (o.id::text = $1 OR o.slug = $1) AND m.user_id = $2
	`, orgID, userID).Scan(&org.ID, &org.Slug, &org.Name, &org.Description, &org.CreatedAt, &role)
	if errors.Is(err, pgx.ErrNoRows) {
		return Organization{}, "", identity.ErrNotFound
	}
	if err != nil {
		return Organization{}, "", err
	}
	org.Role = role
	parsed, ok := rbac.ParseRole(role)
	if !ok {
		return Organization{}, "", identity.ErrForbidden
	}
	return org, parsed, nil
}

func (s *Service) Require(ctx context.Context, userID, orgID string, perm rbac.Permission) (Organization, rbac.Role, error) {
	org, role, err := s.GetOrganization(ctx, userID, orgID)
	if err != nil {
		return Organization{}, "", err
	}
	if !rbac.Can(role, perm) {
		return Organization{}, "", identity.ErrForbidden
	}
	return org, role, nil
}

func (s *Service) ListMembers(ctx context.Context, orgID string) ([]Member, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT m.user_id, u.username, u.display_name, m.role, m.created_at
		FROM organization_members m
		JOIN users u ON u.id = m.user_id
		WHERE m.organization_id = $1
		ORDER BY m.role, u.username
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Member
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.UserID, &m.Username, &m.DisplayName, &m.Role, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Service) AddMember(ctx context.Context, orgID, userID string, role rbac.Role) error {
	if role != rbac.RoleOwner {
		if err := s.ensureNotLastOwner(ctx, orgID, userID); err != nil {
			return err
		}
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO organization_members (organization_id, user_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (organization_id, user_id) DO UPDATE SET role = EXCLUDED.role
	`, orgID, userID, role)
	return err
}

func (s *Service) RemoveMember(ctx context.Context, orgID, userID string) error {
	if err := s.ensureNotLastOwner(ctx, orgID, userID); err != nil {
		return err
	}
	ct, err := s.pool.Exec(ctx, `
		DELETE FROM organization_members WHERE organization_id = $1 AND user_id::text = $2
	`, orgID, userID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return identity.ErrNotFound
	}
	return nil
}

func (s *Service) ensureNotLastOwner(ctx context.Context, orgID, userID string) error {
	var others int
	if err := s.pool.QueryRow(ctx, `
		SELECT count(*) FROM organization_members
		WHERE organization_id = $1 AND role = $2 AND user_id::text <> $3
	`, orgID, rbac.RoleOwner, userID).Scan(&others); err != nil {
		return err
	}
	if others > 0 {
		return nil
	}
	var isOwner bool
	if err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM organization_members
			WHERE organization_id = $1 AND user_id::text = $2 AND role = $3
		)
	`, orgID, userID, rbac.RoleOwner).Scan(&isOwner); err != nil {
		return err
	}
	if isOwner {
		return fmt.Errorf("%w: organization needs at least one owner", identity.ErrInvalidInput)
	}
	return nil
}

func (s *Service) CreateProject(ctx context.Context, actorID, orgID, slug, name, description string) (Project, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if slug == "" || name == "" {
		return Project{}, fmt.Errorf("%w: slug and name are required", identity.ErrInvalidInput)
	}
	var p Project
	err := s.pool.QueryRow(ctx, `
		INSERT INTO projects (organization_id, slug, name, description, created_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, organization_id, slug, name, description, created_at
	`, orgID, slug, name, description, actorID).Scan(&p.ID, &p.OrganizationID, &p.Slug, &p.Name, &p.Description, &p.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "SQLSTATE 23505") {
			return Project{}, identity.ErrConflict
		}
		return Project{}, err
	}
	return p, nil
}

func (s *Service) ListProjects(ctx context.Context, orgID string) ([]Project, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, organization_id, slug, name, description, created_at
		FROM projects
		WHERE organization_id = $1
		ORDER BY slug
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.OrganizationID, &p.Slug, &p.Name, &p.Description, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Service) GetProject(ctx context.Context, orgID, projectID string) (Project, error) {
	projectID = strings.ToLower(strings.TrimSpace(projectID))
	var p Project
	err := s.pool.QueryRow(ctx, `
		SELECT id, organization_id, slug, name, description, created_at
		FROM projects
		WHERE (id::text = $1 OR slug = $1) AND organization_id = $2
	`, projectID, orgID).Scan(&p.ID, &p.OrganizationID, &p.Slug, &p.Name, &p.Description, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Project{}, identity.ErrNotFound
	}
	return p, err
}

func (s *Service) RequireBySlug(ctx context.Context, userID, orgSlug, projectSlug string, perm rbac.Permission) (Organization, Project, error) {
	orgSlug = strings.ToLower(strings.TrimSpace(orgSlug))
	projectSlug = strings.ToLower(strings.TrimSpace(projectSlug))
	var org Organization
	var role string
	err := s.pool.QueryRow(ctx, `
		SELECT o.id, o.slug, o.name, o.description, o.created_at, m.role
		FROM organizations o
		JOIN organization_members m ON m.organization_id = o.id
		WHERE o.slug = $1 AND m.user_id = $2
	`, orgSlug, userID).Scan(&org.ID, &org.Slug, &org.Name, &org.Description, &org.CreatedAt, &role)
	if errors.Is(err, pgx.ErrNoRows) {
		return Organization{}, Project{}, identity.ErrNotFound
	}
	if err != nil {
		return Organization{}, Project{}, err
	}
	parsed, ok := rbac.ParseRole(role)
	if !ok || !rbac.Can(parsed, perm) {
		return Organization{}, Project{}, identity.ErrForbidden
	}
	org.Role = role
	var p Project
	err = s.pool.QueryRow(ctx, `
		SELECT id, organization_id, slug, name, description, created_at
		FROM projects
		WHERE organization_id = $1 AND slug = $2
	`, org.ID, projectSlug).Scan(&p.ID, &p.OrganizationID, &p.Slug, &p.Name, &p.Description, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Organization{}, Project{}, identity.ErrNotFound
	}
	return org, p, err
}
