package tmux

import (
	"fmt"
	"strconv"
	"strings"
)

func ParseSessionID(sessionID string) (string, error) {
	const prefix = "tmux:%"
	if !strings.HasPrefix(sessionID, prefix) {
		return "", NewError(ErrInvalidArgument, "invalid tmux session id: "+sessionID)
	}
	number := strings.TrimPrefix(sessionID, prefix)
	if number == "" {
		return "", NewError(ErrInvalidArgument, "invalid tmux session id: "+sessionID)
	}
	if _, err := strconv.Atoi(number); err != nil {
		return "", NewError(ErrInvalidArgument, "invalid tmux session id: "+sessionID)
	}
	return "%" + number, nil
}

func ParseListPanes(raw string) ([]Session, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []Session{}, nil
	}

	lines := strings.Split(raw, "\n")
	sessions := make([]Session, 0, len(lines))
	for _, line := range lines {
		parts := strings.Split(line, "|")
		if len(parts) < 7 {
			return nil, NewError(ErrBackendError, "invalid tmux list-panes output")
		}
		windowIndex, err := strconv.Atoi(parts[2])
		if err != nil {
			return nil, fmt.Errorf("parse window index: %w", err)
		}
		paneIndex, err := strconv.Atoi(parts[3])
		if err != nil {
			return nil, fmt.Errorf("parse pane index: %w", err)
		}
		sessions = append(sessions, Session{
			ID:             "tmux:" + parts[0],
			Backend:        "tmux",
			Name:           fmt.Sprintf("%s:%d.%d", parts[1], windowIndex, paneIndex),
			SessionName:    parts[1],
			WindowIndex:    windowIndex,
			PaneIndex:      paneIndex,
			Title:          parts[4],
			CWD:            parts[5],
			CurrentCommand: parts[6],
		})
	}
	return sessions, nil
}

func TailBytes(content string, maxBytes int) (string, bool) {
	if maxBytes <= 0 || len([]byte(content)) <= maxBytes {
		return content, false
	}
	bytes := []byte(content)
	return string(bytes[len(bytes)-maxBytes:]), true
}

func CountLines(content string) int {
	if content == "" {
		return 0
	}
	return len(strings.Split(strings.TrimRight(content, "\n"), "\n"))
}
