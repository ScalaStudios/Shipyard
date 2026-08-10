package oidc

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type ProviderConfig struct {
	Name         string
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}

type Service struct {
	providers map[string]ProviderConfig
	mu        sync.Mutex
	states    map[string]stateRecord
}

type stateRecord struct {
	Provider  string
	CreatedAt time.Time
}

func New(providers []ProviderConfig) *Service {
	m := map[string]ProviderConfig{}
	for _, p := range providers {
		if p.Name == "" || p.ClientID == "" || p.Issuer == "" {
			continue
		}
		if len(p.Scopes) == 0 {
			p.Scopes = []string{"openid", "profile", "email"}
		}
		m[p.Name] = p
	}
	return &Service{providers: m, states: map[string]stateRecord{}}
}

func (s *Service) Enabled() bool { return len(s.providers) > 0 }

func (s *Service) List() []string {
	out := make([]string, 0, len(s.providers))
	for name := range s.providers {
		out = append(out, name)
	}
	return out
}

func (s *Service) AuthURL(provider string) (string, string, error) {
	p, ok := s.providers[provider]
	if !ok {
		return "", "", fmt.Errorf("unknown oidc provider")
	}
	state, err := randomState()
	if err != nil {
		return "", "", err
	}
	s.mu.Lock()
	s.states[state] = stateRecord{Provider: provider, CreatedAt: time.Now().UTC()}
	s.mu.Unlock()

	q := url.Values{}
	q.Set("client_id", p.ClientID)
	q.Set("response_type", "code")
	q.Set("scope", strings.Join(p.Scopes, " "))
	q.Set("redirect_uri", p.RedirectURL)
	q.Set("state", state)
	authURL := strings.TrimRight(p.Issuer, "/") + "/protocol/openid-connect/auth"
	if strings.Contains(p.Issuer, "accounts.google.com") {
		authURL = "https://accounts.google.com/o/oauth2/v2/auth"
	}
	if strings.Contains(p.Issuer, "github.com") {
		authURL = "https://github.com/login/oauth/authorize"
	}
	return authURL + "?" + q.Encode(), state, nil
}

type TokenResult struct {
	AccessToken string
	IDToken     string
	Email       string
	Subject     string
	Name        string
}

func (s *Service) Exchange(ctx context.Context, provider, code, state string) (TokenResult, error) {
	s.mu.Lock()
	rec, ok := s.states[state]
	if ok {
		delete(s.states, state)
	}
	s.mu.Unlock()
	if !ok || rec.Provider != provider || time.Since(rec.CreatedAt) > 10*time.Minute {
		return TokenResult{}, errors.New("invalid oauth state")
	}
	p, ok := s.providers[provider]
	if !ok {
		return TokenResult{}, errors.New("unknown provider")
	}

	tokenURL := strings.TrimRight(p.Issuer, "/") + "/protocol/openid-connect/token"
	if strings.Contains(p.Issuer, "accounts.google.com") {
		tokenURL = "https://oauth2.googleapis.com/token"
	}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", p.RedirectURL)
	form.Set("client_id", p.ClientID)
	form.Set("client_secret", p.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return TokenResult{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return TokenResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return TokenResult{}, fmt.Errorf("token exchange failed: %s", resp.Status)
	}
	var payload struct {
		AccessToken string `json:"access_token"`
		IDToken     string `json:"id_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return TokenResult{}, err
	}
	result := TokenResult{AccessToken: payload.AccessToken, IDToken: payload.IDToken}
	if payload.IDToken != "" {
		claims, err := decodeJWTClaims(payload.IDToken)
		if err == nil {
			result.Email, _ = claims["email"].(string)
			result.Subject, _ = claims["sub"].(string)
			result.Name, _ = claims["name"].(string)
		}
	}
	return result, nil
}

func decodeJWTClaims(token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil, errors.New("invalid jwt")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	var claims map[string]any
	if err := json.Unmarshal(raw, &claims); err != nil {
		return nil, err
	}
	return claims, nil
}

func randomState() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
