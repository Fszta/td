package tracker

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Poll reads the tracker's log file and updates the task status.
func (s *Store) Poll(t *Task) {
	if t.Status != StatusRunning && t.Status != StatusNeedsInput {
		return
	}

	processAlive := IsProcessAlive(t.PID)

	f, err := os.Open(s.LogPath(t.ID))
	if err != nil {
		if !processAlive {
			t.Status = StatusCrashed
		}
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 512*1024), 512*1024)

	var hasResult bool

	for scanner.Scan() {
		line := scanner.Bytes()
		var ev event
		if json.Unmarshal(line, &ev) != nil {
			continue
		}

		switch ev.Type {
		case "system":
			if ev.SessionID != "" {
				t.SessionID = ev.SessionID
			}

		case "assistant":
			t.Turns++
			parseToolUse(&ev, t)

		case "result":
			hasResult = true
			t.CostUSD = ev.TotalCostUSD
			t.DurationMs = ev.DurationMs
			t.Turns = ev.NumTurns

			if ev.Subtype == "success" && ev.StopReason == "end_turn" && len(ev.PermissionDenials) == 0 {
				t.Status = StatusDone
			} else if len(ev.PermissionDenials) > 0 {
				t.Status = StatusNeedsInput
				t.Error = "permission denied: " + ev.PermissionDenials[0]
			} else if ev.Subtype != "success" {
				t.Status = StatusFailed
				t.Error = ev.Subtype
			} else {
				t.Status = StatusDone
			}
		}
	}

	if !hasResult && !processAlive && t.Status == StatusRunning {
		t.Status = StatusCrashed
	}
}

func parseToolUse(ev *event, t *Task) {
	if ev.Message == nil {
		return
	}
	for _, c := range ev.Message.Content {
		if c.Type != "tool_use" {
			continue
		}
		t.LastTool = c.Name
		t.LastDetail = extractDetail(c.Name, c.Input)
	}
}

func extractDetail(tool string, input json.RawMessage) string {
	var m map[string]interface{}
	if json.Unmarshal(input, &m) != nil {
		return ""
	}

	switch tool {
	case "Edit", "Write", "Read":
		if p, ok := m["file_path"].(string); ok {
			return filepath.Base(p)
		}
		if p, ok := m["path"].(string); ok {
			return filepath.Base(p)
		}
	case "Bash":
		if cmd, ok := m["command"].(string); ok {
			parts := strings.Fields(cmd)
			if len(parts) > 3 {
				return strings.Join(parts[:3], " ") + "…"
			}
			return cmd
		}
	case "Grep", "Glob":
		if p, ok := m["pattern"].(string); ok {
			return p
		}
	}
	return ""
}

// event is a minimal representation of Claude's stream-json output.
type event struct {
	Type              string   `json:"type"`
	Subtype           string   `json:"subtype,omitempty"`
	SessionID         string   `json:"session_id,omitempty"`
	Message           *message `json:"message,omitempty"`
	TotalCostUSD      float64  `json:"total_cost_usd,omitempty"`
	DurationMs        int64    `json:"duration_ms,omitempty"`
	NumTurns          int      `json:"num_turns,omitempty"`
	StopReason        string   `json:"stop_reason,omitempty"`
	PermissionDenials []string `json:"permission_denials,omitempty"`
}

type message struct {
	Content []contentBlock `json:"content"`
}

type contentBlock struct {
	Type  string          `json:"type"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}
