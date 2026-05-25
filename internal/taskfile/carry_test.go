package taskfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGenerateTemplate(t *testing.T) {
	date := time.Date(2026, 4, 8, 9, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		carried     []Task
		wantContain []string
		wantAbsent  []string
	}{
		{
			name:        "empty day",
			carried:     nil,
			wantContain: []string{"# Wednesday, April 8 2026", "## Tasks", "## Notes"},
			wantAbsent:  []string{"## Carried over"},
		},
		{
			name: "with carried tasks",
			carried: []Task{
				{Text: "unfinished", Status: Open, Tag: TagAgent, Target: "repo"},
			},
			wantContain: []string{"## Carried over", "@agent:repo unfinished", "## Tasks"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := generateTemplate(date, tt.carried)
			for _, s := range tt.wantContain {
				if !strings.Contains(out, s) {
					t.Errorf("missing %q in:\n%s", s, out)
				}
			}
			for _, s := range tt.wantAbsent {
				if strings.Contains(out, s) {
					t.Errorf("unexpected %q in:\n%s", s, out)
				}
			}
		})
	}
}

func TestFindCarriedTasks(t *testing.T) {
	tests := []struct {
		name      string
		prev      string
		wantCount int
		wantText  []string
	}{
		{
			name: "only open tasks carry",
			prev: `## Tasks
- [ ] Still open
- [x] Already done`,
			wantCount: 1,
			wantText:  []string{"Still open"},
		},
		{
			name:      "no previous file",
			prev:      "",
			wantCount: 0,
		},
		{
			name: `all done yields none`,
			prev: `## Tasks
- [x] Done one`,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			today := filepath.Join(dir, "2026", "W15", "2026-04-08-tue.md")

			if tt.prev != "" {
				prev := filepath.Join(dir, "2026", "W14", "2026-04-07-mon.md")
				if err := os.MkdirAll(filepath.Dir(prev), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(prev, []byte(tt.prev), 0644); err != nil {
					t.Fatal(err)
				}
			}

			got := findCarriedTasks(dir, today)
			if len(got) != tt.wantCount {
				t.Fatalf("count: got %d want %d (%+v)", len(got), tt.wantCount, got)
			}
			for i, text := range tt.wantText {
				if got[i].Text != text {
					t.Errorf("task[%d].Text: got %q want %q", i, got[i].Text, text)
				}
			}
		})
	}
}

func TestFindPreviousFile(t *testing.T) {
	tests := []struct {
		name     string
		files    []string // paths relative to temp dir
		today    string
		wantPrev string
	}{
		{
			name:     "latest before today",
			files:    []string{"2026/W14/2026-04-07-mon.md", "2026/W15/2026-04-08-tue.md"},
			today:    "2026/W15/2026-04-08-tue.md",
			wantPrev: "2026/W14/2026-04-07-mon.md",
		},
		{
			name:     "no earlier files",
			files:    []string{"2026/W14/2026-04-07-mon.md"},
			today:    "2026/W14/2026-04-07-mon.md",
			wantPrev: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, rel := range tt.files {
				p := filepath.Join(dir, rel)
				if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte("# day"), 0644); err != nil {
					t.Fatal(err)
				}
			}

			today := filepath.Join(dir, tt.today)
			var wantPrev string
			if tt.wantPrev != "" {
				wantPrev = filepath.Join(dir, tt.wantPrev)
			}

			got := findPreviousFile(dir, today)
			if got != wantPrev {
				t.Errorf("got %q want %q", got, wantPrev)
			}
		})
	}
}
