package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestCommandTransportListsTools(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{Name: "terminal-mcp-smoke-test"}, nil)
	cmd := exec.CommandContext(ctx, "go", "run", ".")
	cmd.Env = append(cmd.Environ(), "TERMINAL_MCP_AUDIT_LOG_PATH="+filepath.Join(t.TempDir(), "audit.jsonl"))
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("Connect returned error: %v", err)
	}
	defer session.Close()

	result, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}
	if len(result.Tools) != 5 {
		t.Fatalf("len(tools) = %d, want 5", len(result.Tools))
	}
}
