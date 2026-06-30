package main

import (
	"context"
	"fmt"
	"os"

	"github.com/gaoyunchuan/terminal-mcp/internal/audit"
	"github.com/gaoyunchuan/terminal-mcp/internal/config"
	"github.com/gaoyunchuan/terminal-mcp/internal/server"
	"github.com/gaoyunchuan/terminal-mcp/internal/tmux"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	cfg := config.Load()
	srv := server.New(tmux.NewBackend(), audit.NewLogger(cfg.AuditLogPath), cfg)
	if err := srv.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		fmt.Fprintf(os.Stderr, "terminal-mcp: server stopped: %v\n", err)
		os.Exit(1)
	}
}
