package tmux

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestIntegrationTmuxLifecycle(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux 未安装，跳过集成测试")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	sessionName := "terminal-mcp-test-" + strings.ReplaceAll(time.Now().Format("150405.000000000"), ".", "")
	if err := exec.CommandContext(ctx, "tmux", "new-session", "-d", "-s", sessionName, "sh").Run(); err != nil {
		t.Fatalf("create tmux session: %v", err)
	}
	defer exec.Command("tmux", "kill-session", "-t", sessionName).Run()

	backend := NewBackend()
	sessions, err := backend.ListSessions(ctx)
	if err != nil {
		t.Fatalf("ListSessions returned error: %v", err)
	}
	var target string
	for _, session := range sessions {
		if session.SessionName == sessionName {
			target = session.ID
			break
		}
	}
	if target == "" {
		t.Fatalf("test tmux pane not found in sessions: %+v", sessions)
	}

	if _, err := backend.SendText(ctx, target, "printf 'hello-terminal-mcp\\n'\n"); err != nil {
		t.Fatalf("SendText returned error: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	read, err := backend.ReadSession(ctx, target, 20, 4096)
	if err != nil {
		t.Fatalf("ReadSession returned error: %v", err)
	}
	if !strings.Contains(read.Content, "hello-terminal-mcp") {
		t.Fatalf("read content = %q", read.Content)
	}

	run, err := backend.RunCommand(ctx, target, "printf 'run-ok'", 2*time.Second, 4096)
	if err != nil {
		t.Fatalf("RunCommand returned error: %v", err)
	}
	if run.Status != "completed" || run.ExitCode == nil || *run.ExitCode != 0 || run.Output != "run-ok" {
		t.Fatalf("run result = %+v", run)
	}

	if _, err := backend.Interrupt(ctx, target); err != nil {
		t.Fatalf("Interrupt returned error: %v", err)
	}
}
