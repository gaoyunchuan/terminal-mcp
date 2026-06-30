package config

import (
	"os"
	"path/filepath"
	"strconv"
)

const (
	defaultTimeoutMS = 30000
	maxTimeoutMS     = 300000
	readMaxBytes     = 65536
	runMaxBytes      = 131072
)

type Config struct {
	AuditLogPath     string
	DefaultTimeoutMS int
	MaxTimeoutMS     int
	ReadMaxBytes     int
	RunMaxBytes      int
}

func Load() Config {
	cfg := Config{
		AuditLogPath:     defaultAuditLogPath(),
		DefaultTimeoutMS: defaultTimeoutMS,
		MaxTimeoutMS:     maxTimeoutMS,
		ReadMaxBytes:     readMaxBytes,
		RunMaxBytes:      runMaxBytes,
	}

	if v := os.Getenv("TERMINAL_MCP_AUDIT_LOG_PATH"); v != "" {
		cfg.AuditLogPath = v
	}
	cfg.MaxTimeoutMS = envInt("TERMINAL_MCP_MAX_TIMEOUT_MS", cfg.MaxTimeoutMS)
	cfg.DefaultTimeoutMS = envInt("TERMINAL_MCP_DEFAULT_TIMEOUT_MS", cfg.DefaultTimeoutMS)
	cfg.ReadMaxBytes = envInt("TERMINAL_MCP_READ_MAX_BYTES", cfg.ReadMaxBytes)
	cfg.RunMaxBytes = envInt("TERMINAL_MCP_RUN_MAX_BYTES", cfg.RunMaxBytes)

	if cfg.MaxTimeoutMS <= 0 {
		cfg.MaxTimeoutMS = maxTimeoutMS
	}
	if cfg.DefaultTimeoutMS <= 0 {
		cfg.DefaultTimeoutMS = defaultTimeoutMS
	}
	if cfg.DefaultTimeoutMS > cfg.MaxTimeoutMS {
		cfg.DefaultTimeoutMS = cfg.MaxTimeoutMS
	}
	if cfg.ReadMaxBytes <= 0 {
		cfg.ReadMaxBytes = readMaxBytes
	}
	if cfg.RunMaxBytes <= 0 {
		cfg.RunMaxBytes = runMaxBytes
	}

	return cfg
}

func defaultAuditLogPath() string {
	home := os.Getenv("HOME")
	if home == "" {
		if detected, err := os.UserHomeDir(); err == nil {
			home = detected
		}
	}
	return filepath.Join(home, ".terminal-mcp", "audit.jsonl")
}

func envInt(name string, fallback int) int {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}
