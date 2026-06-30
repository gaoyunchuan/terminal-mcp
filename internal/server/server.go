package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/gaoyunchuan/terminal-mcp/internal/audit"
	"github.com/gaoyunchuan/terminal-mcp/internal/config"
	"github.com/gaoyunchuan/terminal-mcp/internal/tmux"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Backend interface {
	ListSessions(context.Context) ([]tmux.Session, error)
	ReadSession(context.Context, string, int, int) (tmux.ReadResult, error)
	SendText(context.Context, string, string) (tmux.SendResult, error)
	RunCommand(context.Context, string, string, time.Duration, int) (tmux.RunResult, error)
	Interrupt(context.Context, string) (tmux.InterruptResult, error)
}

type app struct {
	backend Backend
	audit   *audit.Logger
	cfg     config.Config
}

type emptyInput struct{}

type listSessionsOutput struct {
	Sessions []tmux.Session `json:"sessions"`
}

type readSessionInput struct {
	SessionID  string `json:"session_id" jsonschema:"terminal session ID, for example tmux:%1"`
	LastNLines int    `json:"last_n_lines,omitempty" jsonschema:"number of recent lines to read; default 200"`
}

type sendTextInput struct {
	SessionID string `json:"session_id" jsonschema:"terminal session ID, for example tmux:%1"`
	Text      string `json:"text" jsonschema:"raw text to send; newline is not appended automatically"`
}

type runCommandInput struct {
	SessionID string `json:"session_id" jsonschema:"terminal session ID, for example tmux:%1"`
	Command   string `json:"command" jsonschema:"shell command to execute in the target pane"`
	TimeoutMS int    `json:"timeout_ms,omitempty" jsonschema:"maximum wait time in milliseconds; default 30000"`
}

type interruptInput struct {
	SessionID string `json:"session_id" jsonschema:"terminal session ID, for example tmux:%1"`
}

type toolError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func New(backend Backend, logger *audit.Logger, cfg config.Config) *mcp.Server {
	a := &app{backend: backend, audit: logger, cfg: cfg}
	s := mcp.NewServer(&mcp.Implementation{Name: "terminal-mcp", Version: "0.1.0"}, nil)

	readOnly := true
	notDestructive := false
	closedWorld := false

	mcp.AddTool[emptyInput, map[string]any](s, &mcp.Tool{
		Name:        "terminal_list_sessions",
		Title:       "List terminal sessions",
		Description: "Return the tmux panes currently available as terminal sessions.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, DestructiveHint: &notDestructive, OpenWorldHint: &closedWorld},
	}, a.listSessions)
	mcp.AddTool[readSessionInput, map[string]any](s, &mcp.Tool{
		Name:        "terminal_read_session",
		Title:       "Read terminal session",
		Description: "Read recent output from a tmux pane without changing terminal state.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: readOnly, DestructiveHint: &notDestructive, OpenWorldHint: &closedWorld},
	}, a.readSession)
	mcp.AddTool[sendTextInput, map[string]any](s, &mcp.Tool{
		Name:        "terminal_send_text",
		Title:       "Send text to terminal",
		Description: "Send raw text to a tmux pane without appending a newline or waiting for output.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: nil, OpenWorldHint: &closedWorld},
	}, a.sendText)
	mcp.AddTool[runCommandInput, map[string]any](s, &mcp.Tool{
		Name:        "terminal_run_command",
		Title:       "Run terminal command",
		Description: "Run a shell command in a tmux pane and return output, exit code, duration, and timeout state.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: nil, OpenWorldHint: &closedWorld},
	}, a.runCommand)
	mcp.AddTool[interruptInput, map[string]any](s, &mcp.Tool{
		Name:        "terminal_interrupt",
		Title:       "Interrupt terminal session",
		Description: "Send one Ctrl-C interrupt to a tmux pane.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: nil, OpenWorldHint: &closedWorld},
	}, a.interrupt)

	return s
}

func (a *app) listSessions(ctx context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, map[string]any, error) {
	entry := a.newAuditEntry("terminal_list_sessions")
	start := time.Now()
	sessions, err := a.backend.ListSessions(ctx)
	entry.DurationMS = time.Since(start).Milliseconds()
	if err != nil {
		return a.finishError(entry, err)
	}
	entry.Status = "completed"
	output := listSessionsOutput{Sessions: sessions}
	a.writeAudit(entry)
	return nil, mustMap(output), nil
}

func (a *app) readSession(ctx context.Context, _ *mcp.CallToolRequest, in readSessionInput) (*mcp.CallToolResult, map[string]any, error) {
	entry := a.newAuditEntry("terminal_read_session")
	entry.SessionID = in.SessionID
	start := time.Now()
	result, err := a.backend.ReadSession(ctx, in.SessionID, in.LastNLines, a.cfg.ReadMaxBytes)
	entry.DurationMS = time.Since(start).Milliseconds()
	if err != nil {
		return a.finishError(entry, err)
	}
	entry.Status = "completed"
	entry.OutputBytes = len([]byte(result.Content))
	entry.Truncated = result.Truncated
	a.writeAudit(entry)
	return nil, mustMap(result), nil
}

func (a *app) sendText(ctx context.Context, _ *mcp.CallToolRequest, in sendTextInput) (*mcp.CallToolResult, map[string]any, error) {
	entry := a.newAuditEntry("terminal_send_text")
	entry.SessionID = in.SessionID
	entry.TextBytes = len([]byte(in.Text))
	start := time.Now()
	result, err := a.backend.SendText(ctx, in.SessionID, in.Text)
	entry.DurationMS = time.Since(start).Milliseconds()
	if err != nil {
		return a.finishError(entry, err)
	}
	entry.Status = "completed"
	a.writeAudit(entry)
	return nil, mustMap(result), nil
}

func (a *app) runCommand(ctx context.Context, _ *mcp.CallToolRequest, in runCommandInput) (*mcp.CallToolResult, map[string]any, error) {
	entry := a.newAuditEntry("terminal_run_command")
	entry.SessionID = in.SessionID
	entry.Command = in.Command
	timeoutMS := in.TimeoutMS
	if timeoutMS <= 0 {
		timeoutMS = a.cfg.DefaultTimeoutMS
	}
	if timeoutMS > a.cfg.MaxTimeoutMS {
		timeoutMS = a.cfg.MaxTimeoutMS
	}

	start := time.Now()
	result, err := a.backend.RunCommand(ctx, in.SessionID, in.Command, time.Duration(timeoutMS)*time.Millisecond, a.cfg.RunMaxBytes)
	entry.DurationMS = time.Since(start).Milliseconds()
	if err != nil {
		return a.finishError(entry, err)
	}
	entry.Status = result.Status
	entry.ExitCode = result.ExitCode
	entry.OutputBytes = len([]byte(result.Output))
	entry.Truncated = result.Truncated
	if result.Status == "timeout" {
		entry.ErrorCode = string(tmux.ErrCommandTimeout)
		entry.ErrorMessage = "command timed out"
	}
	a.writeAudit(entry)
	return nil, mustMap(result), nil
}

func (a *app) interrupt(ctx context.Context, _ *mcp.CallToolRequest, in interruptInput) (*mcp.CallToolResult, map[string]any, error) {
	entry := a.newAuditEntry("terminal_interrupt")
	entry.SessionID = in.SessionID
	start := time.Now()
	result, err := a.backend.Interrupt(ctx, in.SessionID)
	entry.DurationMS = time.Since(start).Milliseconds()
	if err != nil {
		return a.finishError(entry, err)
	}
	entry.Status = "completed"
	a.writeAudit(entry)
	return nil, mustMap(result), nil
}

func (a *app) newAuditEntry(tool string) audit.Entry {
	return audit.Entry{
		Timestamp: time.Now(),
		RequestID: requestID(),
		Tool:      tool,
	}
}

func (a *app) finishError(entry audit.Entry, err error) (*mcp.CallToolResult, map[string]any, error) {
	entry.Status = "error"
	response := errorResponse(err)
	errObj := response["error"].(map[string]any)
	entry.ErrorCode = errObj["code"].(string)
	entry.ErrorMessage = errObj["message"].(string)
	a.writeAudit(entry)
	return &mcp.CallToolResult{IsError: true}, response, nil
}

func (a *app) writeAudit(entry audit.Entry) {
	if err := a.audit.Write(entry); err != nil {
		fmt.Fprintf(os.Stderr, "terminal-mcp: write audit log: %v\n", err)
	}
}

func errorResponse(err error) map[string]any {
	code := string(tmux.ErrInternal)
	message := err.Error()
	var tmuxErr *tmux.Error
	if errors.As(err, &tmuxErr) {
		code = string(tmuxErr.Code)
		message = tmuxErr.Message
	}
	return map[string]any{"error": map[string]any{"code": code, "message": message}}
}

func mustMap(value any) map[string]any {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		panic(err)
	}
	return out
}

func requestID() string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return "req_" + hex.EncodeToString(bytes[:])
	}
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}
