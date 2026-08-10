package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/oidc"
)

type Config struct {
	HTTPAddr             string
	DatabaseURL          string
	MigrationsDir        string
	StorageBackend       string
	StorageFilesystemDir string
	S3Endpoint           string
	S3Region             string
	S3Bucket             string
	S3AccessKey          string
	S3SecretKey          string
	S3ForcePathStyle     bool
	LogLevel             string
	ShutdownTimeout      time.Duration
	AllowRegister        bool
	SessionTTL           time.Duration
	NodeID               string
	SecretsKey           string
	WebhookSecret        string
	PublicURL            string
	OIDCIssuer           string
	OIDCClientID         string
	OIDCClientSecret     string
	OIDCRedirectURL      string
	OIDCProviderName     string
	OIDCProviders        []oidc.ProviderConfig
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:             envOr("SHIPYARD_HTTP_ADDR", ":8080"),
		DatabaseURL:          os.Getenv("SHIPYARD_DATABASE_URL"),
		MigrationsDir:        envOr("SHIPYARD_MIGRATIONS_DIR", "migrations"),
		StorageBackend:       strings.ToLower(envOr("SHIPYARD_STORAGE_BACKEND", "filesystem")),
		StorageFilesystemDir: envOr("SHIPYARD_STORAGE_DIR", "data/blobs"),
		S3Endpoint:           os.Getenv("SHIPYARD_S3_ENDPOINT"),
		S3Region:             envOr("SHIPYARD_S3_REGION", "us-east-1"),
		S3Bucket:             os.Getenv("SHIPYARD_S3_BUCKET"),
		S3AccessKey:          os.Getenv("SHIPYARD_S3_ACCESS_KEY"),
		S3SecretKey:          os.Getenv("SHIPYARD_S3_SECRET_KEY"),
		S3ForcePathStyle:     envBool("SHIPYARD_S3_FORCE_PATH_STYLE", true),
		LogLevel:             envOr("SHIPYARD_LOG_LEVEL", "info"),
		ShutdownTimeout:      envDuration("SHIPYARD_SHUTDOWN_TIMEOUT", 15*time.Second),
		AllowRegister:        envBool("SHIPYARD_ALLOW_REGISTER", false),
		SessionTTL:           envDuration("SHIPYARD_SESSION_TTL", 7*24*time.Hour),
		NodeID:               envOr("SHIPYARD_NODE_ID", ""),
		SecretsKey:           os.Getenv("SHIPYARD_SECRETS_KEY"),
		WebhookSecret:        os.Getenv("SHIPYARD_WEBHOOK_SECRET"),
		PublicURL:            envOr("SHIPYARD_PUBLIC_URL", "http://127.0.0.1:5173"),
		OIDCIssuer:           os.Getenv("SHIPYARD_OIDC_ISSUER"),
		OIDCClientID:         os.Getenv("SHIPYARD_OIDC_CLIENT_ID"),
		OIDCClientSecret:     os.Getenv("SHIPYARD_OIDC_CLIENT_SECRET"),
		OIDCRedirectURL:      envOr("SHIPYARD_OIDC_REDIRECT_URL", "http://127.0.0.1:8080/api/v1/auth/oidc/primary/callback"),
		OIDCProviderName:     envOr("SHIPYARD_OIDC_PROVIDER", "primary"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("SHIPYARD_DATABASE_URL is required")
	}

	switch cfg.StorageBackend {
	case "filesystem", "s3":
	default:
		return Config{}, fmt.Errorf("unsupported SHIPYARD_STORAGE_BACKEND %q", cfg.StorageBackend)
	}

	if cfg.StorageBackend == "s3" {
		if cfg.S3Bucket == "" {
			return Config{}, fmt.Errorf("SHIPYARD_S3_BUCKET is required when storage backend is s3")
		}
	}

	cfg.OIDCProviders = loadOIDCProviders(cfg)
	return cfg, nil
}

func loadOIDCProviders(cfg Config) []oidc.ProviderConfig {
	var out []oidc.ProviderConfig
	if cfg.OIDCIssuer != "" && cfg.OIDCClientID != "" {
		out = append(out, oidc.ProviderConfig{
			Name:         cfg.OIDCProviderName,
			Kind:         oidc.KindOIDC,
			Issuer:       cfg.OIDCIssuer,
			ClientID:     cfg.OIDCClientID,
			ClientSecret: cfg.OIDCClientSecret,
			RedirectURL:  cfg.OIDCRedirectURL,
		})
	}
	baseRedirect := strings.TrimSuffix(cfg.OIDCRedirectURL, "/"+cfg.OIDCProviderName+"/callback")
	if !strings.Contains(baseRedirect, "/api/v1/auth/oidc") {
		baseRedirect = "http://127.0.0.1:8080/api/v1/auth/oidc"
	}

	if id := os.Getenv("SHIPYARD_OIDC_GITHUB_CLIENT_ID"); id != "" {
		out = append(out, oidc.ProviderConfig{
			Name:         "github",
			Kind:         oidc.KindGitHub,
			ClientID:     id,
			ClientSecret: os.Getenv("SHIPYARD_OIDC_GITHUB_CLIENT_SECRET"),
			RedirectURL:  envOr("SHIPYARD_OIDC_GITHUB_REDIRECT_URL", baseRedirect+"/github/callback"),
		})
	}
	if id := os.Getenv("SHIPYARD_OIDC_GITLAB_CLIENT_ID"); id != "" {
		out = append(out, oidc.ProviderConfig{
			Name:         "gitlab",
			Kind:         oidc.KindGitLab,
			Issuer:       envOr("SHIPYARD_OIDC_GITLAB_ISSUER", "https://gitlab.com"),
			ClientID:     id,
			ClientSecret: os.Getenv("SHIPYARD_OIDC_GITLAB_CLIENT_SECRET"),
			RedirectURL:  envOr("SHIPYARD_OIDC_GITLAB_REDIRECT_URL", baseRedirect+"/gitlab/callback"),
		})
	}
	if id := os.Getenv("SHIPYARD_OIDC_FORGEJO_CLIENT_ID"); id != "" {
		out = append(out, oidc.ProviderConfig{
			Name:         "forgejo",
			Kind:         oidc.KindForgejo,
			Issuer:       envOr("SHIPYARD_OIDC_FORGEJO_ISSUER", "https://git.lunarlabs.dev"),
			ClientID:     id,
			ClientSecret: os.Getenv("SHIPYARD_OIDC_FORGEJO_CLIENT_SECRET"),
			RedirectURL:  envOr("SHIPYARD_OIDC_FORGEJO_REDIRECT_URL", baseRedirect+"/forgejo/callback"),
		})
	}
	if id := os.Getenv("SHIPYARD_OIDC_GITEA_CLIENT_ID"); id != "" {
		out = append(out, oidc.ProviderConfig{
			Name:         "gitea",
			Kind:         oidc.KindGitea,
			Issuer:       os.Getenv("SHIPYARD_OIDC_GITEA_ISSUER"),
			ClientID:     id,
			ClientSecret: os.Getenv("SHIPYARD_OIDC_GITEA_CLIENT_SECRET"),
			RedirectURL:  envOr("SHIPYARD_OIDC_GITEA_REDIRECT_URL", baseRedirect+"/gitea/callback"),
		})
	}
	return out
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return parsed
}
