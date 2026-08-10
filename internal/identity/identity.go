package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/auth"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrConflict       = errors.New("conflict")
	ErrUnauthorized   = errors.New("unauthorized")
	ErrInvalidInput   = errors.New("invalid input")
	ErrForbidden      = errors.New("forbidden")
)

type User struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	IsActive    bool      `json:"is_active"`
	IsAdmin     bool      `json:"is_admin"`
	CreatedAt   time.Time `json:"created_at"`
}

type Service struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func (s *Service) UserCount(ctx context.Context) (int64, error) {
	var n int64
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n)
	return n, err
}

func (s *Service) CreateUser(ctx context.Context, username, email, displayName, password string) (User, error) {
	username = strings.TrimSpace(username)
	email = strings.TrimSpace(strings.ToLower(email))
	displayName = strings.TrimSpace(displayName)
	if username == "" || email == "" {
		return User{}, fmt.Errorf("%w: username and email are required", ErrInvalidInput)
	}
	if displayName == "" {
		displayName = username
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return User{}, err
	}

	var u User
	err = s.pool.QueryRow(ctx, `
		INSERT INTO users (username, email, display_name, password_hash, is_admin)
		VALUES ($1, $2, $3, $4, NOT EXISTS (SELECT 1 FROM users))
		RETURNING id, username, email, display_name, is_active, is_admin, created_at
	`, username, email, displayName, hash).Scan(&u.ID, &u.Username, &u.Email, &u.DisplayName, &u.IsActive, &u.IsAdmin, &u.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrConflict
		}
		return User{}, err
	}
	return u, nil
}

func (s *Service) Authenticate(ctx context.Context, login, password string) (User, error) {
	login = strings.TrimSpace(login)
	var (
		u    User
		hash string
	)
	err := s.pool.QueryRow(ctx, `
		SELECT id, username, email, display_name, is_active, is_admin, created_at, password_hash
		FROM users
		WHERE lower(username) = lower($1) OR lower(email) = lower($1)
	`, login).Scan(&u.ID, &u.Username, &u.Email, &u.DisplayName, &u.IsActive, &u.IsAdmin, &u.CreatedAt, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUnauthorized
	}
	if err != nil {
		return User{}, err
	}
	if !u.IsActive {
		return User{}, ErrUnauthorized
	}
	ok, err := auth.VerifyPassword(hash, password)
	if err != nil {
		return User{}, err
	}
	if !ok {
		return User{}, ErrUnauthorized
	}
	return u, nil
}

func (s *Service) GetUserByEmail(ctx context.Context, email string) (User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT id, username, email, display_name, is_active, is_admin, created_at
		FROM users WHERE lower(email) = $1
	`, email).Scan(&u.ID, &u.Username, &u.Email, &u.DisplayName, &u.IsActive, &u.IsAdmin, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func (s *Service) FindUser(ctx context.Context, login string) (User, error) {
	login = strings.TrimSpace(login)
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT id, username, email, display_name, is_active, is_admin, created_at
		FROM users
		WHERE lower(username) = lower($1) OR lower(email) = lower($1)
	`, login).Scan(&u.ID, &u.Username, &u.Email, &u.DisplayName, &u.IsActive, &u.IsAdmin, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func (s *Service) EnsureOIDCUser(ctx context.Context, username, email, displayName string) (User, error) {
	if existing, err := s.GetUserByEmail(ctx, email); err == nil {
		return existing, nil
	} else if !errors.Is(err, ErrNotFound) {
		return User{}, err
	}
	password, err := auth.NewToken(24)
	if err != nil {
		return User{}, err
	}
	return s.CreateUser(ctx, username, email, displayName, password)
}

func (s *Service) CreateSession(ctx context.Context, userID string, ttl time.Duration) (token string, expiresAt time.Time, err error) {
	token, err = auth.NewToken(32)
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt = time.Now().UTC().Add(ttl)
	_, err = s.pool.Exec(ctx, `
		INSERT INTO sessions (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, userID, auth.HashToken(token), expiresAt)
	return token, expiresAt, err
}

func (s *Service) UserFromSession(ctx context.Context, token string) (User, error) {
	if token == "" {
		return User{}, ErrUnauthorized
	}
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, u.username, u.email, u.display_name, u.is_active, u.is_admin, u.created_at
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1
		  AND s.revoked_at IS NULL
		  AND s.expires_at > now()
		  AND u.is_active = TRUE
	`, auth.HashToken(token)).Scan(&u.ID, &u.Username, &u.Email, &u.DisplayName, &u.IsActive, &u.IsAdmin, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUnauthorized
	}
	return u, err
}

func (s *Service) RevokeSession(ctx context.Context, token string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE sessions SET revoked_at = now()
		WHERE token_hash = $1 AND revoked_at IS NULL
	`, auth.HashToken(token))
	return err
}

func (s *Service) CreateAPIToken(ctx context.Context, userID, name string, ttl *time.Duration) (plain string, prefix string, expiresAt *time.Time, err error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", "", nil, fmt.Errorf("%w: name is required", ErrInvalidInput)
	}
	plain, err = auth.NewToken(32)
	if err != nil {
		return "", "", nil, err
	}
	prefix = auth.TokenPrefix(plain)
	var exp any
	if ttl != nil {
		t := time.Now().UTC().Add(*ttl)
		expiresAt = &t
		exp = t
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO api_tokens (user_id, name, token_prefix, token_hash, expires_at)
		VALUES ($1, $2, $3, $4, $5)
	`, userID, name, prefix, auth.HashToken(plain), exp)
	return plain, prefix, expiresAt, err
}

func (s *Service) UserFromAPIToken(ctx context.Context, token string) (User, error) {
	if token == "" {
		return User{}, ErrUnauthorized
	}
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, u.username, u.email, u.display_name, u.is_active, u.is_admin, u.created_at
		FROM api_tokens t
		JOIN users u ON u.id = t.user_id
		WHERE t.token_hash = $1
		  AND t.revoked_at IS NULL
		  AND (t.expires_at IS NULL OR t.expires_at > now())
		  AND u.is_active = TRUE
	`, auth.HashToken(token)).Scan(&u.ID, &u.Username, &u.Email, &u.DisplayName, &u.IsActive, &u.IsAdmin, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUnauthorized
	}
	if err != nil {
		return User{}, err
	}
	_, _ = s.pool.Exec(ctx, `UPDATE api_tokens SET last_used_at = now() WHERE token_hash = $1`, auth.HashToken(token))
	return u, nil
}

func isUniqueViolation(err error) bool {
	return strings.Contains(err.Error(), "SQLSTATE 23505")
}
