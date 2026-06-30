package tmux

import "testing"

func TestParseSessionID(t *testing.T) {
	paneID, err := ParseSessionID("tmux:%12")
	if err != nil {
		t.Fatalf("ParseSessionID returned error: %v", err)
	}
	if paneID != "%12" {
		t.Fatalf("paneID = %q, want %%12", paneID)
	}
}

func TestParseSessionIDRejectsInvalidValues(t *testing.T) {
	for _, input := range []string{"", "tmux:", "tmux:12", "screen:%1", "tmux:%"} {
		if _, err := ParseSessionID(input); err == nil {
			t.Fatalf("ParseSessionID(%q) returned nil error", input)
		}
	}
}

func TestParseListPanes(t *testing.T) {
	raw := "%1|work|0|1|agentgrid-dev|/Users/ycgao/code/agentgrid|zsh\n" +
		"%2|work|1|0|tests|/tmp/project|go"

	sessions, err := ParseListPanes(raw)
	if err != nil {
		t.Fatalf("ParseListPanes returned error: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("len(sessions) = %d, want 2", len(sessions))
	}
	first := sessions[0]
	if first.ID != "tmux:%1" || first.Backend != "tmux" || first.Name != "work:0.1" {
		t.Fatalf("unexpected first session: %+v", first)
	}
	if first.SessionName != "work" || first.WindowIndex != 0 || first.PaneIndex != 1 {
		t.Fatalf("unexpected indexes: %+v", first)
	}
	if first.Title != "agentgrid-dev" || first.CWD != "/Users/ycgao/code/agentgrid" || first.CurrentCommand != "zsh" {
		t.Fatalf("unexpected metadata: %+v", first)
	}
}

func TestTailBytesKeepsSuffixAndReportsTruncation(t *testing.T) {
	got, truncated := TailBytes("0123456789", 4)
	if got != "6789" {
		t.Fatalf("TailBytes content = %q, want 6789", got)
	}
	if !truncated {
		t.Fatalf("TailBytes truncated = false, want true")
	}
}
