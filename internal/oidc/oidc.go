package oidc

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type ProviderKind string

const (
	KindOIDC    ProviderKind = "oidc"
	KindGitHub  ProviderKind = "github"
	KindGitLab  ProviderKind = "gitlab"
	KindForgejo ProviderKind = "forgejo"
	KindGitea   ProviderKind = "gitea"
)

type ProviderConfig struct {
	Name         string
	Kind         ProviderKind
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

type ProviderInfo struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

func New(providers []ProviderConfig) *Service {
	m := map[string]ProviderConfig{}
	for _, p := range providers {
		p = Normalize(p)
		if p.Name == "" || p.ClientID == "" {
			continue
		}
		if p.Kind == KindOIDC && p.Issuer == "" {
			continue
		}
		m[p.Name] = p
	}
	return &Service{providers: m, states: map[string]stateRecord{}}
}

func Normalize(p ProviderConfig) ProviderConfig {
	p.Name = strings.ToLower(strings.TrimSpace(p.Name))
	p.Kind = ProviderKind(strings.ToLower(string(p.Kind)))
	if p.Kind == "" {
		p.Kind = detectKind(p)
	}
	switch p.Kind {
	case KindGitHub:
		if p.Issuer == "" {
			p.Issuer = "https://github.com"
		}
		if len(p.Scopes) == 0 {
			p.Scopes = []string{"read:user", "user:email"}
		}
	case KindGitLab:
		if p.Issuer == "" {
			p.Issuer = "https://gitlab.com"
		}
		if len(p.Scopes) == 0 {
			p.Scopes = []string{"openid", "profile", "email"}
		}
	case KindForgejo, KindGitea:
		if len(p.Scopes) == 0 {
			p.Scopes = []string{"openid", "profile", "email"}
		}
	default:
		p.Kind = KindOIDC
		if len(p.Scopes) == 0 {
			p.Scopes = []string{"openid", "profile", "email"}
		}
	}
	return p
}

func detectKind(p ProviderConfig) ProviderKind {
	name := strings.ToLower(p.Name)
	issuer := strings.ToLower(p.Issuer)
	switch {
	case name == "github" || strings.Contains(issuer, "github.com"):
		return KindGitHub
	case name == "gitlab" || strings.Contains(issuer, "gitlab"):
		return KindGitLab
	case name == "forgejo" || strings.Contains(issuer, "forgejo"):
		return KindForgejo
	case name == "gitea" || strings.Contains(issuer, "gitea"):
		return KindGitea
	default:
		return KindOIDC
	}
}

func (s *Service) Enabled() bool { return len(s.providers) > 0 }

func (s *Service) List() []ProviderInfo {
	out := make([]ProviderInfo, 0, len(s.providers))
	for _, p := range s.providers {
		out = append(out, ProviderInfo{Name: p.Name, Kind: string(p.Kind)})
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
	return authEndpoint(p) + "?" + q.Encode(), state, nil
}

type TokenResult struct {
	AccessToken string
	IDToken     string
	Email       string
	Subject     string
	Name        string
	Username    string
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

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", p.RedirectURL)
	form.Set("client_id", p.ClientID)
	form.Set("client_secret", p.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint(p), strings.NewReader(form.Encode()))
	if err != nil {
		return TokenResult{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return TokenResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return TokenResult{}, fmt.Errorf("token exchange failed: %s %s", resp.Status, strings.TrimSpace(string(body)))
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
		if claims, err := decodeJWTClaims(payload.IDToken); err == nil {
			result.Email, _ = claims["email"].(string)
			result.Subject, _ = claims["sub"].(string)
			result.Name, _ = claims["name"].(string)
			result.Username, _ = claims["preferred_username"].(string)
		}
	}
	if result.Email == "" || result.Username == "" {
		_ = enrichFromUserInfo(ctx, p, result.AccessToken, &result)
	}
	return result, nil
}

func enrichFromUserInfo(ctx context.Context, p ProviderConfig, accessToken string, result *TokenResult) error {
	endpoint := userInfoEndpoint(p)
	if endpoint == "" || accessToken == "" {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("userinfo failed: %s", resp.Status)
	}
	var info map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return err
	}
	if result.Email == "" {
		if email, ok := info["email"].(string); ok {
			result.Email = email
		}
	}
	if result.Name == "" {
		if name, ok := info["name"].(string); ok {
			result.Name = name
		}
	}
	if result.Username == "" {
		for _, key := range []string{"login", "username", "preferred_username"} {
			if v, ok := info[key].(string); ok && v != "" {
				result.Username = v
				break
			}
		}
	}
	if result.Subject == "" {
		switch v := info["id"].(type) {
		case string:
			result.Subject = v
		case float64:
			result.Subject = fmt.Sprintf("%.0f", v)
		}
	}
	if result.Email == "" && p.Kind == KindGitHub {
		result.Email = fetchGitHubPrimaryEmail(ctx, accessToken)
	}
	return nil
}

func fetchGitHubPrimaryEmail(ctx context.Context, accessToken string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return ""
	}
	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email
		}
	}
	for _, e := range emails {
		if e.Verified {
			return e.Email
		}
	}
	return ""
}

func authEndpoint(p ProviderConfig) string {
	base := strings.TrimRight(p.Issuer, "/")
	switch p.Kind {
	case KindGitHub:
		return "https://github.com/login/oauth/authorize"
	case KindGitLab:
		return base + "/oauth/authorize"
	case KindForgejo, KindGitea:
		return base + "/login/oauth/authorize"
	default:
		if strings.Contains(base, "accounts.google.com") {
			return "https://accounts.google.com/o/oauth2/v2/auth"
		}
		return base + "/protocol/openid-connect/auth"
	}
}

func tokenEndpoint(p ProviderConfig) string {
	base := strings.TrimRight(p.Issuer, "/")
	switch p.Kind {
	case KindGitHub:
		return "https://github.com/login/oauth/access_token"
	case KindGitLab:
		return base + "/oauth/token"
	case KindForgejo, KindGitea:
		return base + "/login/oauth/access_token"
	default:
		if strings.Contains(base, "accounts.google.com") {
			return "https://oauth2.googleapis.com/token"
		}
		return base + "/protocol/openid-connect/token"
	}
}

func userInfoEndpoint(p ProviderConfig) string {
	base := strings.TrimRight(p.Issuer, "/")
	switch p.Kind {
	case KindGitHub:
		return "https://api.github.com/user"
	case KindGitLab:
		return base + "/api/v4/user"
	case KindForgejo, KindGitea:
		return base + "/api/v1/user"
	default:
		if strings.Contains(base, "accounts.google.com") {
			return "https://openidconnect.googleapis.com/v1/userinfo"
		}
		return base + "/protocol/openid-connect/userinfo"
	}
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
