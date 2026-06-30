package config

import (
	"path/filepath"
	"testing"
)

func TestLoadUsesDefaults(t *testing.T) {
	t.Setenv("HOME", "/Users/tester")
	t.Setenv("TERMINAL_MCP_AUDIT_LOG_PATH", "")
	t.Setenv("TERMINAL_MCP_DEFAULT_TIMEOUT_MS", "")
	t.Setenv("TERMINAL_MCP_MAX_TIMEOUT_MS", "")
	t.Setenv("TERMINAL_MCP_READ_MAX_BYTES", "")
	t.Setenv("TERMINAL_MCP_RUN_MAX_BYTES", "")

	cfg := Load()

	wantAudit := filepath.Join("/Users/tester", ".terminal-mcp", "audit.jsonl")
	if cfg.AuditLogPath != wantAudit {
		t.Fatalf("AuditLogPath = %q, want %q", cfg.AuditLogPath, wantAudit)
	}
	if cfg.DefaultTimeoutMS != 30000 {
		t.Fatalf("DefaultTimeoutMS = %d, want 30000", cfg.DefaultTimeoutMS)
	}
	if cfg.MaxTimeoutMS != 300000 {
		t.Fatalf("MaxTimeoutMS = %d, want 300000", cfg.MaxTimeoutMS)
	}
	if cfg.ReadMaxBytes != 65536 {
		t.Fatalf("ReadMaxBytes = %d, want 65536", cfg.ReadMaxBytes)
	}
	if cfg.RunMaxBytes != 131072 {
		t.Fatalf("RunMaxBytes = %d, want 131072", cfg.RunMaxBytes)
	}
}

func TestLoadAppliesEnvironmentOverridesAndClampsDefaultTimeout(t *testing.T) {
	t.Setenv("TERMINAL_MCP_AUDIT_LOG_PATH", "/tmp/audit.jsonl")
	t.Setenv("TERMINAL_MCP_DEFAULT_TIMEOUT_MS", "500000")
	t.Setenv("TERMINAL_MCP_MAX_TIMEOUT_MS", "120000")
	t.Setenv("TERMINAL_MCP_READ_MAX_BYTES", "1024")
	t.Setenv("TERMINAL_MCP_RUN_MAX_BYTES", "2048")

	cfg := Load()

	if cfg.AuditLogPath != "/tmp/audit.jsonl" {
		t.Fatalf("AuditLogPath = %q", cfg.AuditLogPath)
	}
	if cfg.DefaultTimeoutMS != 120000 {
		t.Fatalf("DefaultTimeoutMS = %d, want clamped 120000", cfg.DefaultTimeoutMS)
	}
	if cfg.MaxTimeoutMS != 120000 {
		t.Fatalf("MaxTimeoutMS = %d, want 120000", cfg.MaxTimeoutMS)
	}
	if cfg.ReadMaxBytes != 1024 || cfg.RunMaxBytes != 2048 {
		t.Fatalf("byte limits = %d/%d, want 1024/2048", cfg.ReadMaxBytes, cfg.RunMaxBytes)
	}
}
