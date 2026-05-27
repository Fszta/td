package tui

import (
	"strings"
	"testing"
)

func TestParseWatchLine_system(t *testing.T) {
	raw := `{"type":"system","session_id":"abc12345","model":"claude-sonnet-4-6"}`
	lines := parseWatchLine(raw)
	if len(lines) == 0 {
		t.Fatal("expected lines")
	}
	if !strings.Contains(lines[0], "Session started") {
		t.Fatalf("got %q", lines[0])
	}
}

func TestParseWatchLine_toolUse(t *testing.T) {
	raw := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"git status"}}]}}`
	lines := parseWatchLine(raw)
	found := false
	for _, l := range lines {
		if strings.Contains(l, "Bash") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected Bash tool line, got %v", lines)
	}
}

func TestParseWatchLine_resultSuccess(t *testing.T) {
	raw := `{"type":"result","subtype":"success","num_turns":3,"total_cost_usd":0.012,"duration_ms":45000}`
	lines := parseWatchLine(raw)
	found := false
	for _, l := range lines {
		if strings.Contains(l, "Done") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected Done line, got %v", lines)
	}
}

func TestParseWatchLine_resultFailure(t *testing.T) {
	raw := `{"type":"result","subtype":"error","error":"context deadline exceeded"}`
	lines := parseWatchLine(raw)
	found := false
	for _, l := range lines {
		if strings.Contains(l, "Failed") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected Failed line, got %v", lines)
	}
	// Error message should appear in the output.
	hasMsg := false
	for _, l := range lines {
		if strings.Contains(l, "context deadline exceeded") {
			hasMsg = true
		}
	}
	if !hasMsg {
		t.Fatalf("expected error message in output, got %v", lines)
	}
}

func TestParseWatchLine_invalidJSON(t *testing.T) {
	if lines := parseWatchLine("not json"); lines != nil {
		t.Fatalf("expected nil, got %v", lines)
	}
}
