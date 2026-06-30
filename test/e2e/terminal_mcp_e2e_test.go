//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const vmSSH = "root@vm"

type listSessionsOutput struct {
	Sessions []terminalSession `json:"sessions"`
}

type terminalSession struct {
	ID          string `json:"id"`
	SessionName string `json:"session_name"`
}

type readSessionOutput struct {
	Content string `json:"content"`
}

type sendTextOutput struct {
	Accepted bool `json:"accepted"`
}

type runCommandOutput struct {
	Status   string `json:"status"`
	ExitCode *int   `json:"exit_code"`
	Output   string `json:"output"`
}

type interruptOutput struct {
	Interrupted bool `json:"interrupted"`
}

type auditEntry struct {
	Tool   string `json:"tool"`
	Status string `json:"status"`
}

func TestTerminalMCPEndToEndAgainstVM(t *testing.T) {
	requireCommand(t, "tmux")
	requireSSHVM(t)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	repoRoot := findRepoRoot(t)
	auditPath := filepath.Join(t.TempDir(), "audit.jsonl")
	sessionName := "terminal-mcp-e2e-" + time.Now().Format("150405000000000")
	createTmuxSession(t, ctx, sessionName)
	defer exec.Command("tmux", "kill-session", "-t", sessionName).Run()

	clientSession := connectMCPServer(t, ctx, repoRoot, auditPath)
	defer clientSession.Close()

	targetID := findSessionID(t, ctx, clientSession, sessionName)

	send := callTool[sendTextOutput](t, ctx, clientSession, "terminal_send_text", map[string]any{
		"session_id": targetID,
		"text":       "printf 'terminal-mcp-e2e-send\\n'\n",
	})
	if !send.Accepted {
		t.Fatalf("terminal_send_text accepted = false")
	}
	waitForPaneContent(t, ctx, clientSession, targetID, "terminal-mcp-e2e-send")

	vmHost := strings.TrimSpace(runLocal(t, ctx, "ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=10", vmSSH, "hostname"))
	run := callTool[runCommandOutput](t, ctx, clientSession, "terminal_run_command", map[string]any{
		"session_id": targetID,
		"command":    "ssh -o BatchMode=yes -o ConnectTimeout=10 " + vmSSH + " 'printf terminal-mcp-e2e-vm:; hostname'",
		"timeout_ms": 20000,
	})
	if run.Status != "completed" || run.ExitCode == nil || *run.ExitCode != 0 {
		t.Fatalf("run remote command result = %+v", run)
	}
	if !strings.Contains(run.Output, "terminal-mcp-e2e-vm:"+vmHost) {
		t.Fatalf("run remote output = %q, want host %q", run.Output, vmHost)
	}

	failed := callTool[runCommandOutput](t, ctx, clientSession, "terminal_run_command", map[string]any{
		"session_id": targetID,
		"command":    "ssh -o BatchMode=yes -o ConnectTimeout=10 " + vmSSH + " 'exit 7'",
		"timeout_ms": 20000,
	})
	if failed.Status != "completed" || failed.ExitCode == nil || *failed.ExitCode != 7 {
		t.Fatalf("run failing remote command result = %+v", failed)
	}

	_ = callTool[sendTextOutput](t, ctx, clientSession, "terminal_send_text", map[string]any{
		"session_id": targetID,
		"text":       "ssh -o BatchMode=yes -o ConnectTimeout=10 " + vmSSH + " 'sleep 30'\n",
	})
	time.Sleep(500 * time.Millisecond)
	interrupted := callTool[interruptOutput](t, ctx, clientSession, "terminal_interrupt", map[string]any{
		"session_id": targetID,
	})
	if !interrupted.Interrupted {
		t.Fatalf("terminal_interrupt interrupted = false")
	}
	time.Sleep(500 * time.Millisecond)

	timeout := callTool[runCommandOutput](t, ctx, clientSession, "terminal_run_command", map[string]any{
		"session_id": targetID,
		"command":    "ssh -o BatchMode=yes -o ConnectTimeout=10 " + vmSSH + " 'sleep 5; echo too-late'",
		"timeout_ms": 500,
	})
	if timeout.Status != "timeout" || timeout.ExitCode != nil {
		t.Fatalf("run timeout result = %+v", timeout)
	}
	_ = callTool[interruptOutput](t, ctx, clientSession, "terminal_interrupt", map[string]any{"session_id": targetID})

	assertAuditHasTools(t, auditPath, []string{
		"terminal_list_sessions",
		"terminal_send_text",
		"terminal_read_session",
		"terminal_run_command",
		"terminal_interrupt",
	})
}

func requireCommand(t *testing.T, name string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		t.Fatalf("%s 不可用: %v", name, err)
	}
}

func requireSSHVM(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=10", vmSSH, "hostname").CombinedOutput()
	if err != nil {
		t.Fatalf("ssh %s 不可用: %v\n%s", vmSSH, err, out)
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			t.Fatalf("找不到 go.mod")
		}
		wd = parent
	}
}

func createTmuxSession(t *testing.T, ctx context.Context, sessionName string) {
	t.Helper()
	cmd := exec.CommandContext(ctx, "tmux", "new-session", "-d", "-s", sessionName, "sh")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create tmux session: %v\n%s", err, out)
	}
}

func connectMCPServer(t *testing.T, ctx context.Context, repoRoot string, auditPath string) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "terminal-mcp-e2e"}, nil)
	cmd := exec.CommandContext(ctx, "go", "run", "./cmd/terminal-mcp")
	cmd.Dir = repoRoot
	cmd.Env = append(cmd.Environ(), "TERMINAL_MCP_AUDIT_LOG_PATH="+auditPath)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("connect MCP server: %v", err)
	}
	return session
}

func findSessionID(t *testing.T, ctx context.Context, clientSession *mcp.ClientSession, sessionName string) string {
	t.Helper()
	out := callTool[listSessionsOutput](t, ctx, clientSession, "terminal_list_sessions", map[string]any{})
	for _, session := range out.Sessions {
		if session.SessionName == sessionName {
			return session.ID
		}
	}
	t.Fatalf("找不到 tmux session %q: %+v", sessionName, out.Sessions)
	return ""
}

func waitForPaneContent(t *testing.T, ctx context.Context, clientSession *mcp.ClientSession, sessionID string, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		read := callTool[readSessionOutput](t, ctx, clientSession, "terminal_read_session", map[string]any{
			"session_id":   sessionID,
			"last_n_lines": 50,
		})
		if strings.Contains(read.Content, want) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("pane output did not contain %q", want)
}

func callTool[T any](t *testing.T, ctx context.Context, clientSession *mcp.ClientSession, name string, args map[string]any) T {
	t.Helper()
	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s protocol error: %v", name, err)
	}
	if result.IsError {
		data, _ := json.Marshal(result.StructuredContent)
		t.Fatalf("%s tool error: %s", name, data)
	}
	var out T
	data, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("%s marshal structured content: %v", name, err)
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("%s unmarshal structured content: %v\n%s", name, err, data)
	}
	return out
}

func runLocal(t *testing.T, ctx context.Context, name string, args ...string) string {
	t.Helper()
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, out)
	}
	return string(out)
}

func assertAuditHasTools(t *testing.T, path string, tools []string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var entry auditEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("audit line is not JSON: %v\n%s", err, line)
		}
		seen[entry.Tool] = true
	}
	for _, tool := range tools {
		if !seen[tool] {
			t.Fatalf("audit log missing %s; seen=%v", tool, seen)
		}
	}
}
