package tracker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExtractDetail(t *testing.T) {
	tests := []struct {
		name string
		tool string
		in   string
		want string
	}{
		{"edit file_path", "Edit", `{"file_path":"/proj/internal/foo.go"}`, "foo.go"},
		{"read path", "Read", `{"path":"/tmp/bar.txt"}`, "bar.txt"},
		{"bash short command", "Bash", `{"command":"git status --short"}`, "git status --short"},
		{"bash truncates long command", "Bash", `{"command":"one two three four five six"}`, "one two three…"},
		{"grep pattern", "Grep", `{"pattern":"func Parse"}`, "func Parse"},
		{"unknown tool", "Unknown", `{}`, ""},
		{"invalid json", "Edit", `{not json`, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractDetail(tt.tool, []byte(tt.in))
			if got != tt.want {
				t.Errorf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestPoll(t *testing.T) {
	alivePID := os.Getpid()

	tests := []struct {
		name           string
		events         string
		pid            int
		initialStatus  Status
		wantStatus     Status
		wantSession    string
		wantLastTool   string
		wantLastDetail string
		wantTurns      int
		wantCost       float64
		wantError      string
	}{
		{
			name: "success with tool use",
			events: `{"type":"system","session_id":"sess-1"}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Edit","input":{"file_path":"/x/y.go"}}]}}
{"type":"result","subtype":"success","stop_reason":"end_turn","total_cost_usd":0.01,"duration_ms":1000,"num_turns":2}`,
			pid:            alivePID,
			initialStatus:  StatusRunning,
			wantStatus:     StatusDone,
			wantSession:    "sess-1",
			wantLastTool:   "Edit",
			wantLastDetail: "y.go",
			wantTurns:      2,
			wantCost:       0.01,
		},
		{
			name: "permission denied",
			events: `{"type":"result","subtype":"success","stop_reason":"end_turn","permission_denials":["write outside workspace"]}`,
			pid:           alivePID,
			initialStatus: StatusRunning,
			wantStatus:    StatusNeedsInput,
			wantError:     "permission denied",
		},
		{
			name:          "failed subtype",
			events:        `{"type":"result","subtype":"error"}`,
			pid:           alivePID,
			initialStatus: StatusRunning,
			wantStatus:    StatusFailed,
			wantError:     "error",
		},
		{
			name:          "crashed when no log and dead pid",
			events:        "",
			pid:           999999999,
			initialStatus: StatusRunning,
			wantStatus:    StatusCrashed,
		},
		{
			name:          "ignores poll when not running",
			events:        `{"type":"result","subtype":"success","stop_reason":"end_turn"}`,
			pid:           alivePID,
			initialStatus: StatusDone,
			wantStatus:    StatusDone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			store := &Store{dir: dir}
			task := &Task{
				ID:     "poll-" + tt.name,
				Status: tt.initialStatus,
				PID:    tt.pid,
			}
			if tt.events != "" {
				if err := os.WriteFile(store.LogPath(task.ID), []byte(tt.events), 0644); err != nil {
					t.Fatal(err)
				}
			}

			store.Poll(task)

			if task.Status != tt.wantStatus {
				t.Errorf("status: got %q want %q", task.Status, tt.wantStatus)
			}
			if tt.wantSession != "" && task.SessionID != tt.wantSession {
				t.Errorf("session: got %q want %q", task.SessionID, tt.wantSession)
			}
			if tt.wantLastTool != "" && task.LastTool != tt.wantLastTool {
				t.Errorf("last tool: got %q want %q", task.LastTool, tt.wantLastTool)
			}
			if tt.wantLastDetail != "" && task.LastDetail != tt.wantLastDetail {
				t.Errorf("last detail: got %q want %q", task.LastDetail, tt.wantLastDetail)
			}
			if tt.wantTurns != 0 && task.Turns != tt.wantTurns {
				t.Errorf("turns: got %d want %d", task.Turns, tt.wantTurns)
			}
			if tt.wantCost != 0 && task.CostUSD != tt.wantCost {
				t.Errorf("cost: got %v want %v", task.CostUSD, tt.wantCost)
			}
			if tt.wantError != "" && !strings.Contains(task.Error, tt.wantError) {
				t.Errorf("error: got %q want substring %q", task.Error, tt.wantError)
			}
		})
	}
}

func TestStore(t *testing.T) {
	dir := t.TempDir()
	store := &Store{dir: dir}

	t1 := &Task{ID: "t1", Status: StatusRunning, StartedAt: parseTime(t, "2026-04-08T10:00:00Z")}
	t2 := &Task{ID: "t2", Status: StatusDone, StartedAt: parseTime(t, "2026-04-09T10:00:00Z")}

	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "register and get",
			run: func(t *testing.T) {
				if err := store.Register(t1); err != nil {
					t.Fatal(err)
				}
				if got := store.Get("t1"); got == nil || got.ID != "t1" {
					t.Fatalf("Get: %+v", got)
				}
			},
		},
		{
			name: "active tasks",
			run: func(t *testing.T) {
				if err := store.Register(t2); err != nil {
					t.Fatal(err)
				}
				active := store.ActiveTasks()
				if len(active) != 1 || active[0].ID != "t1" {
					t.Errorf("ActiveTasks: %+v", active)
				}
			},
		},
		{
			name: "list recent",
			run: func(t *testing.T) {
				recent := store.ListRecent(1)
				if len(recent) != 1 || recent[0].ID != "t2" {
					t.Errorf("ListRecent: %+v", recent)
				}
			},
		},
		{
			name: "remove task and log",
			run: func(t *testing.T) {
				_ = os.WriteFile(store.LogPath("t1"), []byte("log"), 0644)
				if err := store.Remove("t1"); err != nil {
					t.Fatal(err)
				}
				if store.Get("t1") != nil {
					t.Error("task should be removed")
				}
				if _, err := os.Stat(filepath.Join(dir, "t1.jsonl")); !os.IsNotExist(err) {
					t.Error("log file should be removed")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

func parseTime(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return ts
}
