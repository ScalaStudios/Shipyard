package secrets

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/identity"
)

type Box struct {
	pool *pgxpool.Pool
	gcm  cipher.AEAD
}

func New(pool *pgxpool.Pool, keyB64 string) (*Box, error) {
	if keyB64 == "" {
		keyB64 = os.Getenv("SHIPYARD_SECRETS_KEY")
	}
	if keyB64 == "" {
		return nil, fmt.Errorf("SHIPYARD_SECRETS_KEY is required for secrets")
	}
	raw, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil || len(raw) != 32 {
		return nil, fmt.Errorf("SHIPYARD_SECRETS_KEY must be base64-encoded 32 bytes")
	}
	block, err := aes.NewCipher(raw)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{pool: pool, gcm: gcm}, nil
}

type SecretMeta struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Scope  string `json:"scope"`
}

func (b *Box) Put(ctx context.Context, orgID, projectID, envID, actorID, name, value string) (SecretMeta, error) {
	name = strings.TrimSpace(name)
	if name == "" || value == "" {
		return SecretMeta{}, fmt.Errorf("%w: name and value required", identity.ErrInvalidInput)
	}
	nonce := make([]byte, b.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return SecretMeta{}, err
	}
	ct := b.gcm.Seal(nil, nonce, []byte(value), nil)
	var id string
	err := b.pool.QueryRow(ctx, `
		INSERT INTO secrets (organization_id, project_id, environment_id, name, ciphertext, nonce, created_by)
		VALUES (NULLIF($1,'')::uuid, NULLIF($2,'')::uuid, NULLIF($3,'')::uuid, $4, $5, $6, $7)
		RETURNING id
	`, orgID, projectID, envID, name, ct, nonce, actorID).Scan(&id)
	if err != nil {
		if strings.Contains(err.Error(), "SQLSTATE 23505") {
			return SecretMeta{}, identity.ErrConflict
		}
		return SecretMeta{}, err
	}
	return SecretMeta{ID: id, Name: name, Scope: scopeOf(orgID, projectID, envID)}, nil
}

func (b *Box) GetValue(ctx context.Context, secretID string) (string, error) {
	var ct, nonce []byte
	err := b.pool.QueryRow(ctx, `SELECT ciphertext, nonce FROM secrets WHERE id = $1`, secretID).Scan(&ct, &nonce)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", identity.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	pt, err := b.gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

func (b *Box) ValuesForScope(ctx context.Context, orgID, projectID string) ([]string, error) {
	named, err := b.NamedValuesForScope(ctx, orgID, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(named))
	for _, v := range named {
		out = append(out, v)
	}
	return out, nil
}

func (b *Box) NamedValuesForScope(ctx context.Context, orgID, projectID string) (map[string]string, error) {
	rows, err := b.pool.Query(ctx, `
		SELECT name, ciphertext, nonce FROM secrets
		WHERE organization_id = $1::uuid
		  AND (project_id IS NULL OR project_id = NULLIF($2,'')::uuid)
	`, orgID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var name string
		var ct, nonce []byte
		if err := rows.Scan(&name, &ct, &nonce); err != nil {
			return nil, err
		}
		pt, err := b.gcm.Open(nil, nonce, ct, nil)
		if err != nil {
			continue
		}
		if v := string(pt); v != "" {
			out[name] = v
		}
	}
	return out, rows.Err()
}

func MaskLine(line string, secrets []string) string {
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		line = strings.ReplaceAll(line, secret, "***")
	}
	return line
}

func EnvName(secretName string) string {
	up := strings.ToUpper(strings.TrimSpace(secretName))
	var b strings.Builder
	for _, r := range up {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	name := b.String()
	if name == "" {
		return "SHIPYARD_SECRET"
	}
	if name[0] >= '0' && name[0] <= '9' {
		return "S_" + name
	}
	return name
}

func scopeOf(orgID, projectID, envID string) string {
	switch {
	case envID != "":
		return "environment"
	case projectID != "":
		return "project"
	default:
		return "organization"
	}
}
