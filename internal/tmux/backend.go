package tmux

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const listPanesFormat = "#{pane_id}|#{session_name}|#{window_index}|#{pane_index}|#{pane_title}|#{pane_current_path}|#{pane_current_command}"
const runCommandCaptureStart = "-10000"

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
	paneID, err := ParseSessionID(sessionID)
	if err != nil {
		return RunResult{}, err
	}
	start := time.Now()
	id := strconv.FormatInt(start.UnixNano(), 36)
	beginMarker := "__TERMINAL_MCP_BEGIN_" + id + "__"
	doneMarker := "__TERMINAL_MCP_DONE_" + id + "__"
	script := buildRunScript(command, beginMarker, doneMarker)

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
			output, truncated, _, _ := b.readRunCapture(ctx, paneID, beginMarker, doneMarker, maxBytes)
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
			output, truncated, code, ok := b.readRunCapture(ctx, paneID, beginMarker, doneMarker, maxBytes)
			if !ok {
				continue
			}
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

func (b *Backend) readRunCapture(ctx context.Context, paneID, beginMarker, doneMarker string, maxBytes int) (string, bool, int, bool) {
	out, err := b.run(ctx, "", "capture-pane", "-p", "-J", "-t", paneID, "-S", runCommandCaptureStart)
	if err != nil {
		return "", false, 0, false
	}
	return parseRunCapture(string(out), beginMarker, doneMarker, maxBytes)
}

func buildRunScript(command, beginMarker, doneMarker string) string {
	return fmt.Sprintf("__terminal_mcp_cmd=%s; printf '\\n%%s\\n' %s; eval \"$__terminal_mcp_cmd\" 2>&1; __terminal_mcp_code=$?; printf '\\n%s:%%s\\n' \"$__terminal_mcp_code\"",
		shellQuote(command),
		shellQuote(beginMarker),
		doneMarker,
	)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func parseRunCapture(content, beginMarker, doneMarker string, maxBytes int) (string, bool, int, bool) {
	_, beginLineEnd, ok := findExactLine(content, beginMarker)
	if !ok {
		return "", false, 0, false
	}
	outputStart := beginLineEnd
	if outputStart < len(content) && content[outputStart] == '\n' {
		outputStart++
	}

	doneSearchStart := outputStart
	donePrefix := doneMarker + ":"
	for doneSearchStart <= len(content) {
		lineStart, lineEnd := nextLine(content, doneSearchStart)
		line := strings.TrimSuffix(content[lineStart:lineEnd], "\r")
		if strings.HasPrefix(line, donePrefix) {
			code, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, donePrefix)))
			if err != nil {
				return "", false, 0, false
			}
			outputEnd := lineStart
			if outputEnd > outputStart && content[outputEnd-1] == '\n' {
				outputEnd--
			}
			output, truncated := TailBytes(content[outputStart:outputEnd], maxBytes)
			return output, truncated, code, true
		}
		if lineEnd == len(content) {
			break
		}
		doneSearchStart = lineEnd + 1
	}

	output, truncated := TailBytes(content[outputStart:], maxBytes)
	return output, truncated, 0, false
}

func findExactLine(content, want string) (int, int, bool) {
	for start := 0; start <= len(content); {
		lineStart, lineEnd := nextLine(content, start)
		line := strings.TrimSuffix(content[lineStart:lineEnd], "\r")
		if line == want {
			return lineStart, lineEnd, true
		}
		if lineEnd == len(content) {
			break
		}
		start = lineEnd + 1
	}
	return 0, 0, false
}

func nextLine(content string, start int) (int, int) {
	if start > len(content) {
		return len(content), len(content)
	}
	relativeEnd := strings.IndexByte(content[start:], '\n')
	if relativeEnd < 0 {
		return start, len(content)
	}
	return start, start + relativeEnd
}
