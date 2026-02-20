package main

import (
	"log/slog"
	"os"
	"path/filepath"
)

// Config holds application configuration loaded from environment variables.
type Config struct {
	Mode        string     // "stdio" or "http"
	HTTPAddr    string     // "127.0.0.1:8080"
	DBDriver    string     // "sqlite" or "postgres"
	DBDSN       string     // file path or connection string
	AgeKeyPath  string     // path to age identity file
	ConfigFile  string     // path to mcplexer.yaml
	LogLevel    slog.Level // slog level
	SocketPath  string     // unix socket path for multi-client mode
	ExternalURL string     // external URL for OAuth callbacks
}

// defaultDataPath returns ~/.mcplexer/<filename>, falling back to
// a CWD-relative path if the home directory can't be resolved.
func defaultDataPath(filename string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filename
	}
	return filepath.Join(home, ".mcplexer", filename)
}

func loadConfig() (*Config, error) {
	cfg := &Config{
		Mode:        envOr("MCPLEXER_MODE", "stdio"),
		HTTPAddr:    envOr("MCPLEXER_HTTP_ADDR", "127.0.0.1:8080"),
		DBDriver:    envOr("MCPLEXER_DB_DRIVER", "sqlite"),
		DBDSN:       envOr("MCPLEXER_DB_DSN", defaultDataPath("mcplexer.db")),
		AgeKeyPath:  envOr("MCPLEXER_AGE_KEY", ""),
		ConfigFile:  envOr("MCPLEXER_CONFIG", defaultDataPath("mcplexer.yaml")),
		LogLevel:    parseLogLevel(envOr("MCPLEXER_LOG_LEVEL", "info")),
		SocketPath:  envOr("MCPLEXER_SOCKET_PATH", ""),
		ExternalURL: envOr("MCPLEXER_EXTERNAL_URL", ""),
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseLogLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
