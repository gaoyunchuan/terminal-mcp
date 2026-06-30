package server

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/gaoyunchuan/terminal-mcp/internal/audit"
	"github.com/gaoyunchuan/terminal-mcp/internal/config"
	"github.com/gaoyunchuan/terminal-mcp/internal/tmux"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type fakeBackend struct{}

func (fakeBackend) ListSessions(context.Context) ([]tmux.Session, error) {
	return []tmux.Session{{ID: "tmux:%1", Backend: "tmux", Name: "work:0.1", SessionName: "work"}}, nil
}

func (fakeBackend) ReadSession(context.Context, string, int, int) (tmux.ReadResult, error) {
	return tmux.ReadResult{}, tmux.NewError(tmux.ErrSessionNotFound, "session not found: tmux:%99")
}

func (fakeBackend) SendText(context.Context, string, string) (tmux.SendResult, error) {
	return tmux.SendResult{SessionID: "tmux:%1", Accepted: true, BytesSent: 3}, nil
}

func (fakeBackend) RunCommand(context.Context, string, string, time.Duration, int) (tmux.RunResult, error) {
	code := 0
	return tmux.RunResult{SessionID: "tmux:%1", Command: "pwd", Status: "completed", ExitCode: &code, Output: "/tmp", DurationMS: 1}, nil
}

func (fakeBackend) Interrupt(context.Context, string) (tmux.InterruptResult, error) {
	return tmux.InterruptResult{SessionID: "tmux:%1", Interrupted: true}, nil
}

func TestServerRegistersTerminalTools(t *testing.T) {
	session := connectTestServer(t)
	defer session.Close()

	result, err := session.ListTools(context.Background(), &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}
	names := map[string]bool{}
	for _, tool := range result.Tools {
		names[tool.Name] = true
	}
	for _, want := range []string{"terminal_list_sessions", "terminal_read_session", "terminal_send_text", "terminal_run_command", "terminal_interrupt"} {
		if !names[want] {
			t.Fatalf("tool %q not registered; got %v", want, names)
		}
	}
}

func TestServerReturnsStructuredToolErrorAndWritesAudit(t *testing.T) {
	session := connectTestServer(t)
	defer session.Close()

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "terminal_read_session",
		Arguments: map[string]any{"session_id": "tmux:%99"},
	})
	if err != nil {
		t.Fatalf("CallTool returned protocol error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("IsError = false, want true")
	}
	structured, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("StructuredContent = %T, want map", result.StructuredContent)
	}
	errorObject := structured["error"].(map[string]any)
	if errorObject["code"] != "SESSION_NOT_FOUND" {
		t.Fatalf("error code = %v", errorObject["code"])
	}
}

func TestServerListSessionsReturnsStructuredContent(t *testing.T) {
	session := connectTestServer(t)
	defer session.Close()

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "terminal_list_sessions"})
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("IsError = true, want false")
	}
	data, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("Marshal structured content: %v", err)
	}
	if !json.Valid(data) {
		t.Fatalf("structured content is not JSON")
	}
}

func connectTestServer(t *testing.T) *mcp.ClientSession {
	t.Helper()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	cfg := config.Config{
		AuditLogPath:     filepath.Join(t.TempDir(), "audit.jsonl"),
		DefaultTimeoutMS: 30000,
		MaxTimeoutMS:     300000,
		ReadMaxBytes:     65536,
		RunMaxBytes:      131072,
	}
	srv := New(fakeBackend{}, audit.NewLogger(cfg.AuditLogPath), cfg)
	go func() {
		_ = srv.Run(context.Background(), serverTransport)
	}()
	client := mcp.NewClient(&mcp.Implementation{Name: "terminal-mcp-test"}, nil)
	session, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatalf("Connect returned error: %v", err)
	}
	return session
}
