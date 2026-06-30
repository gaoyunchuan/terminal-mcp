package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoggerWriteCreatesJSONL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "audit.jsonl")
	logger := NewLogger(path)

	entry := Entry{
		Timestamp:    time.Date(2026, 6, 30, 11, 30, 0, 0, time.FixedZone("CST", 8*60*60)),
		RequestID:    "req_test",
		Tool:         "terminal_run_command",
		SessionID:    "tmux:%1",
		Command:      "pwd",
		Status:       "completed",
		ExitCode:     intPtr(0),
		DurationMS:   42,
		OutputBytes:  3,
		Truncated:    false,
		ErrorCode:    "",
		ErrorMessage: "",
	}

	if err := logger.Write(entry); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	var decoded Entry
	if err := json.Unmarshal(data[:len(data)-1], &decoded); err != nil {
		t.Fatalf("audit line is not JSON: %v", err)
	}
	if decoded.RequestID != "req_test" || decoded.Tool != "terminal_run_command" {
		t.Fatalf("decoded entry = %+v", decoded)
	}
}

func intPtr(v int) *int {
	return &v
}
