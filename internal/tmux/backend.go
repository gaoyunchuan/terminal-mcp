package tmux

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const listPanesFormat = "#{pane_id}|#{session_name}|#{window_index}|#{pane_index}|#{pane_title}|#{pane_current_path}|#{pane_current_command}"

type runnerFunc func(ctx context.Context, stdin string, args ...string) ([]byte, error)

type Backend struct {
	run runnerFunc
}

func NewBackend() *Backend {
	return &Backend{run: execTmux}
}

func newBackendForTest(run runnerFunc) *Backend {
	return &Backend{run: run}
}

func (b *Backend) ListSessions(ctx context.Context) ([]Session, error) {
	out, err := b.run(ctx, "", "list-panes", "-a", "-F", listPanesFormat)
	if err != nil {
		if isNoServer(out) {
			return []Session{}, nil
		}
		return nil, mapBackendError(out, err, "")
	}
	return ParseListPanes(string(out))
}

func (b *Backend) ReadSession(ctx context.Context, sessionID string, lastNLines int, maxBytes int) (ReadResult, error) {
	paneID, err := ParseSessionID(sessionID)
	if err != nil {
		return ReadResult{}, err
	}
	if lastNLines <= 0 {
		lastNLines = 200
	}
	out, err := b.run(ctx, "", "capture-pane", "-p", "-t", paneID, "-S", "-"+strconv.Itoa(lastNLines))
	if err != nil {
		return ReadResult{}, mapBackendError(out, err, sessionID)
	}
	content, truncated := TailBytes(string(out), maxBytes)
	return ReadResult{
		SessionID: sessionID,
		Content:   content,
		LineCount: CountLines(content),
		Truncated: truncated,
	}, nil
}

func (b *Backend) SendText(ctx context.Context, sessionID string, text string) (SendResult, error) {
	paneID, err := ParseSessionID(sessionID)
	if err != nil {
		return SendResult{}, err
	}
	bufferName := "terminal-mcp-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	if out, err := b.run(ctx, text, "load-buffer", "-b", bufferName, "-"); err != nil {
		return SendResult{}, mapBackendError(out, err, sessionID)
	}
	if out, err := b.run(ctx, "", "paste-buffer", "-b", bufferName, "-t", paneID, "-d"); err != nil {
		return SendResult{}, mapBackendError(out, err, sessionID)
	}
	return SendResult{SessionID: sessionID, Accepted: true, BytesSent: len([]byte(text))}, nil
}

func (b *Backend) Interrupt(ctx context.Context, sessionID string) (InterruptResult, error) {
	paneID, err := ParseSessionID(sessionID)
	if err != nil {
		return InterruptResult{}, err
	}
	if out, err := b.run(ctx, "", "send-keys", "-t", paneID, "C-c"); err != nil {
		return InterruptResult{}, mapBackendError(out, err, sessionID)
	}
	return InterruptResult{SessionID: sessionID, Interrupted: true}, nil
}

func (b *Backend) RunCommand(ctx context.Context, sessionID string, command string, timeout time.Duration, maxBytes int) (RunResult, error) {
	if strings.TrimSpace(command) == "" || strings.ContainsRune(command, '\x00') {
		return RunResult{}, NewError(ErrInvalidArgument, "command must be non-empty text without NUL bytes")
	}
	start := time.Now()
	id := strconv.FormatInt(start.UnixNano(), 36)
	outputPath := filepath.Join(os.TempDir(), "terminal-mcp-"+id+".out")
	statusPath := filepath.Join(os.TempDir(), "terminal-mcp-"+id+".status")
	marker := "__TERMINAL_MCP_DONE_" + id + "__"
	script := buildRunScript(command, outputPath, statusPath, marker)

	if _, err := b.SendText(ctx, sessionID, script+"\n"); err != nil {
		return RunResult{}, err
	}

	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return RunResult{}, NewError(ErrInternal, ctx.Err().Error())
		case <-deadline.C:
			output, truncated := readTailFile(outputPath, maxBytes)
			return RunResult{
				SessionID:  sessionID,
				Command:    command,
				Status:     "timeout",
				ExitCode:   nil,
				Output:     output,
				DurationMS: time.Since(start).Milliseconds(),
				Truncated:  truncated,
			}, nil
		case <-ticker.C:
			code, ok := readExitCode(statusPath)
			if !ok {
				continue
			}
			output, truncated := readTailFile(outputPath, maxBytes)
			_ = os.Remove(outputPath)
			_ = os.Remove(statusPath)
			return RunResult{
				SessionID:  sessionID,
				Command:    command,
				Status:     "completed",
				ExitCode:   &code,
				Output:     output,
				DurationMS: time.Since(start).Milliseconds(),
				Truncated:  truncated,
			}, nil
		}
	}
}

func execTmux(ctx context.Context, stdin string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "tmux", args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined
	err := cmd.Run()
	if errors.Is(err, exec.ErrNotFound) {
		return combined.Bytes(), NewError(ErrBackendUnavailable, "tmux command not found")
	}
	return combined.Bytes(), err
}

func mapBackendError(out []byte, err error, sessionID string) error {
	if err == nil {
		return nil
	}
	var tmuxErr *Error
	if errors.As(err, &tmuxErr) {
		return tmuxErr
	}
	message := strings.TrimSpace(string(out))
	if message == "" {
		message = err.Error()
	}
	if strings.Contains(message, "can't find pane") || strings.Contains(message, "can't find window") {
		if sessionID == "" {
			return NewError(ErrSessionNotFound, message)
		}
		return NewError(ErrSessionNotFound, "session not found: "+sessionID)
	}
	return NewError(ErrBackendError, message)
}

func isNoServer(out []byte) bool {
	return strings.Contains(string(out), "no server running")
}

func buildRunScript(command, outputPath, statusPath, marker string) string {
	return fmt.Sprintf("__terminal_mcp_out=%s; __terminal_mcp_status=%s; __terminal_mcp_cmd=%s; eval \"$__terminal_mcp_cmd\" > \"$__terminal_mcp_out\" 2>&1; __terminal_mcp_code=$?; printf '%%s\\n' \"$__terminal_mcp_code\" > \"$__terminal_mcp_status\"; printf '\\n%s:%%s\\n' \"$__terminal_mcp_code\"; (sleep 300; rm -f \"$__terminal_mcp_out\" \"$__terminal_mcp_status\") >/dev/null 2>&1 &",
		shellQuote(outputPath),
		shellQuote(statusPath),
		shellQuote(command),
		marker,
	)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func readExitCode(path string) (int, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	code, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, false
	}
	return code, true
}

func readTailFile(path string, maxBytes int) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return TailBytes(string(data), maxBytes)
}
