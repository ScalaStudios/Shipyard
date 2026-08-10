package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"git.lunarlabs.dev/Shipyard/shipyard/internal/config"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/database"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/httpapi"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/logging"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/secrets"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/settings"
	"git.lunarlabs.dev/Shipyard/shipyard/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		logging.New("error").Error("config load failed", "error", err)
		os.Exit(1)
	}

	log := logging.New(cfg.LogLevel)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("database connect failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := database.Migrate(ctx, pool, cfg.MigrationsDir); err != nil {
		log.Error("migrations failed", "error", err)
		os.Exit(1)
	}

	var admins int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE is_admin`).Scan(&admins); err == nil && admins == 0 {
		var users int
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&users)
		if users > 0 {
			log.Warn("no instance admin exists; grant one with: UPDATE users SET is_admin = TRUE WHERE username = '<you>'", "users", users)
		}
	}

	store, err := newStore(cfg)
	if err != nil {
		log.Error("storage init failed", "error", err)
		os.Exit(1)
	}

	if cfg.SecretsKey != "" {
		box, err := secrets.New(pool, cfg.SecretsKey)
		if err != nil {
			log.Error("secrets init failed", "error", err)
			os.Exit(1)
		}
		n, err := box.BackfillLegacy(ctx)
		if err != nil {
			log.Error("encrypting stored credentials failed", "error", err)
			os.Exit(1)
		}
		if n > 0 {
			log.Info("encrypted plaintext credentials at rest", "values", n)
		}
		seeded, err := settings.New(pool, box).SeedProvidersFromEnv(ctx, cfg.OIDCProviders, cfg.ForgeOAuthProviders)
		if err != nil {
			log.Error("seeding auth providers from env failed", "error", err)
			os.Exit(1)
		}
		if seeded > 0 {
			log.Info("imported auth providers from environment; manage them in settings from now on", "providers", seeded)
		}
	} else {
		log.Warn("SHIPYARD_SECRETS_KEY is not set; storing forge, scm and discord credentials will be rejected")
	}

	opts := httpapi.Options{
		AllowRegister: cfg.AllowRegister,
		SessionTTL:    cfg.SessionTTL,
		NodeID:        cfg.NodeID,
		SecretsKey:    cfg.SecretsKey,
		WebhookSecret: cfg.WebhookSecret,
		PublicURL:     cfg.PublicURL,
		APIURL:        cfg.APIURL,
		OIDC:          cfg.OIDCProviders,
		ForgeOAuth:    cfg.ForgeOAuthProviders,
	}
	api := httpapi.New(pool, store, opts)
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("shipyard-server listening", "addr", cfg.HTTPAddr)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Error("shutdown failed", "error", err)
			os.Exit(1)
		}
		log.Info("shipyard-server stopped")
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server failed", "error", err)
			os.Exit(1)
		}
	}
}

func newStore(cfg config.Config) (storage.Store, error) {
	switch cfg.StorageBackend {
	case "filesystem":
		return storage.NewFilesystemStore(cfg.StorageFilesystemDir)
	case "s3":
		return storage.NewS3Store(storage.S3Config{
			Endpoint:       cfg.S3Endpoint,
			Region:         cfg.S3Region,
			Bucket:         cfg.S3Bucket,
			AccessKey:      cfg.S3AccessKey,
			SecretKey:      cfg.S3SecretKey,
			ForcePathStyle: cfg.S3ForcePathStyle,
		})
	default:
		return nil, errors.New("unknown storage backend")
	}
}
