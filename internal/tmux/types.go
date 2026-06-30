package tmux

type ErrorCode string

const (
	ErrSessionNotFound    ErrorCode = "SESSION_NOT_FOUND"
	ErrInvalidArgument    ErrorCode = "INVALID_ARGUMENT"
	ErrBackendUnavailable ErrorCode = "BACKEND_UNAVAILABLE"
	ErrBackendError       ErrorCode = "BACKEND_ERROR"
	ErrCommandTimeout     ErrorCode = "COMMAND_TIMEOUT"
	ErrOutputTooLarge     ErrorCode = "OUTPUT_TOO_LARGE"
	ErrInternal           ErrorCode = "INTERNAL_ERROR"
)

type Error struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

func (e *Error) Error() string {
	return string(e.Code) + ": " + e.Message
}

func NewError(code ErrorCode, message string) *Error {
	return &Error{Code: code, Message: message}
}

type Session struct {
	ID             string `json:"id"`
	Backend        string `json:"backend"`
	Name           string `json:"name"`
	SessionName    string `json:"session_name"`
	WindowIndex    int    `json:"window_index"`
	PaneIndex      int    `json:"pane_index"`
	Title          string `json:"title,omitempty"`
	CWD            string `json:"cwd,omitempty"`
	CurrentCommand string `json:"current_command,omitempty"`
}

type ReadResult struct {
	SessionID string `json:"session_id"`
	Content   string `json:"content"`
	LineCount int    `json:"line_count"`
	Truncated bool   `json:"truncated"`
}

type SendResult struct {
	SessionID string `json:"session_id"`
	Accepted  bool   `json:"accepted"`
	BytesSent int    `json:"bytes_sent"`
}

type RunResult struct {
	SessionID  string `json:"session_id"`
	Command    string `json:"command"`
	Status     string `json:"status"`
	ExitCode   *int   `json:"exit_code"`
	Output     string `json:"output"`
	DurationMS int64  `json:"duration_ms"`
	Truncated  bool   `json:"truncated"`
}

type InterruptResult struct {
	SessionID   string `json:"session_id"`
	Interrupted bool   `json:"interrupted"`
}
