package githubapp

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Config struct {
	AppID         string
	Slug          string
	PrivateKeyPEM string
	APIBaseURL    string
}

type Installation struct {
	ID        int64  `json:"id"`
	Account   string `json:"account"`
	AvatarURL string `json:"avatar_url"`
}

func (c Config) apiBase() string {
	base := strings.TrimRight(strings.TrimSpace(c.APIBaseURL), "/")
	if base == "" {
		return "https://api.github.com"
	}
	return base
}

func (c Config) Ready() bool {
	return strings.TrimSpace(c.AppID) != "" && strings.TrimSpace(c.PrivateKeyPEM) != ""
}

func parsePrivateKey(pemData string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(pemData)))
	if block == nil {
		return nil, fmt.Errorf("private key is not valid PEM")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is not RSA")
	}
	return key, nil
}

func AppJWT(cfg Config, now time.Time) (string, error) {
	if !cfg.Ready() {
		return "", fmt.Errorf("github app is not configured")
	}
	key, err := parsePrivateKey(cfg.PrivateKeyPEM)
	if err != nil {
		return "", err
	}
	header := map[string]string{"alg": "RS256", "typ": "JWT"}
	claims := map[string]any{
		"iat": now.Add(-30 * time.Second).Unix(),
		"exp": now.Add(9 * time.Minute).Unix(),
		"iss": strings.TrimSpace(cfg.AppID),
	}
	segments := make([]string, 0, 3)
	for _, part := range []any{header, claims} {
		raw, err := json.Marshal(part)
		if err != nil {
			return "", err
		}
		segments = append(segments, base64.RawURLEncoding.EncodeToString(raw))
	}
	signing := strings.Join(segments, ".")
	digest := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return signing + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func InstallationToken(ctx context.Context, cfg Config, installationID string) (string, time.Time, error) {
	token, err := AppJWT(cfg, time.Now().UTC())
	if err != nil {
		return "", time.Time{}, err
	}
	endpoint := fmt.Sprintf("%s/app/installations/%s/access_tokens", cfg.apiBase(), strings.TrimSpace(installationID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return "", time.Time{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", time.Time{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return "", time.Time{}, fmt.Errorf("github app token %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var payload struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", time.Time{}, err
	}
	if payload.Token == "" {
		return "", time.Time{}, fmt.Errorf("github app returned an empty installation token")
	}
	return payload.Token, payload.ExpiresAt, nil
}

func ListInstallations(ctx context.Context, cfg Config) ([]Installation, error) {
	token, err := AppJWT(cfg, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.apiBase()+"/app/installations?per_page=100", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github app installations %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var raw []struct {
		ID      int64 `json:"id"`
		Account struct {
			Login     string `json:"login"`
			AvatarURL string `json:"avatar_url"`
		} `json:"account"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	out := make([]Installation, 0, len(raw))
	for _, item := range raw {
		out = append(out, Installation{ID: item.ID, Account: item.Account.Login, AvatarURL: item.Account.AvatarURL})
	}
	return out, nil
}

func InstallURL(cfg Config, state string) string {
	slug := strings.TrimSpace(cfg.Slug)
	if slug == "" {
		return ""
	}
	host := "https://github.com"
	if base := cfg.apiBase(); base != "https://api.github.com" {
		host = strings.TrimSuffix(strings.TrimSuffix(base, "/api/v3"), "/")
	}
	url := host + "/apps/" + slug + "/installations/new"
	if state != "" {
		url += "?state=" + state
	}
	return url
}
