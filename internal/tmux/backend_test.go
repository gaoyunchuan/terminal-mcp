package tmux

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestBackendListSessionsUsesTmuxListPanes(t *testing.T) {
	var gotArgs []string
	backend := newBackendForTest(func(_ context.Context, stdin string, args ...string) ([]byte, error) {
		if stdin != "" {
			t.Fatalf("stdin = %q, want empty", stdin)
		}
		gotArgs = append([]string{}, args...)
		return []byte("%1|work|0|1|title|/tmp|zsh\n"), nil
	})

	sessions, err := backend.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("ListSessions returned error: %v", err)
	}
	if strings.Join(gotArgs, " ") != "list-panes -a -F #{pane_id}|#{session_name}|#{window_index}|#{pane_index}|#{pane_title}|#{pane_current_path}|#{pane_current_command}" {
		t.Fatalf("tmux args = %q", gotArgs)
	}
	if len(sessions) != 1 || sessions[0].ID != "tmux:%1" {
		t.Fatalf("sessions = %+v", sessions)
	}
}

func TestBackendListSessionsReturnsEmptyWhenNoTmuxServer(t *testing.T) {
	backend := newBackendForTest(func(context.Context, string, ...string) ([]byte, error) {
		return []byte("no server running on /tmp/tmux-501/default"), errors.New("exit status 1")
	})

	sessions, err := backend.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("ListSessions returned error: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("len(sessions) = %d, want 0", len(sessions))
	}
}

func TestBackendReadSessionCapturesTail(t *testing.T) {
	backend := newBackendForTest(func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if strings.Join(args, " ") != "capture-pane -p -t %1 -S -2" {
			t.Fatalf("tmux args = %q", args)
		}
		return []byte("line1\nline2\nline3"), nil
	})

	result, err := backend.ReadSession(context.Background(), "tmux:%1", 2, 9)
	if err != nil {
		t.Fatalf("ReadSession returned error: %v", err)
	}
	if result.Content != "ne2\nline3" || !result.Truncated {
		t.Fatalf("result = %+v", result)
	}
}

func TestBackendRunCommandTimeoutReturnsPartialOutput(t *testing.T) {
	backend := newBackendForTest(func(context.Context, string, ...string) ([]byte, error) {
		return nil, nil
	})

	result, err := backend.RunCommand(context.Background(), "tmux:%1", "sleep 10", 5*time.Millisecond, 1024)
	if err != nil {
		t.Fatalf("RunCommand returned error: %v", err)
	}
	if result.Status != "timeout" || result.ExitCode != nil {
		t.Fatalf("result = %+v", result)
	}
}
