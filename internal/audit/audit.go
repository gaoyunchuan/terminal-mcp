package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Entry struct {
	Timestamp    time.Time `json:"timestamp"`
	RequestID    string    `json:"request_id"`
	Tool         string    `json:"tool"`
	SessionID    string    `json:"session_id,omitempty"`
	Command      string    `json:"command,omitempty"`
	TextBytes    int       `json:"text_bytes,omitempty"`
	Status       string    `json:"status"`
	ExitCode     *int      `json:"exit_code,omitempty"`
	DurationMS   int64     `json:"duration_ms"`
	OutputBytes  int       `json:"output_bytes,omitempty"`
	Truncated    bool      `json:"truncated,omitempty"`
	ErrorCode    string    `json:"error_code,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty"`
}

type Logger struct {
	path string
	mu   sync.Mutex
}

func NewLogger(path string) *Logger {
	return &Logger{path: path}
}

func (l *Logger) Write(entry Entry) error {
	if l == nil || l.path == "" {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	encoded, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(encoded, '\n')); err != nil {
		return err
	}
	return nil
}
