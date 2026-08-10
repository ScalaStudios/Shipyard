package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
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

	return cfg, nil
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
