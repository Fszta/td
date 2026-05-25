package tracker

import (
	"strings"
	"testing"
	"time"
)

func TestBranchName(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		want      string
		maxLen    int // 0 = exact match only
	}{
		{"simple phrase", "Create Snowflake task", "td/create-snowflake-task", 0},
		{"normalizes whitespace and case", "  Mixed   CASE  ", "td/mixed-case", 0},
		{"empty input", "", "td/", 0},
		{
			name:   "truncates long slug",
			in:     strings.Repeat("word-", 20) + "end",
			maxLen: len("td/") + 60,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BranchName(tt.in)
			if tt.maxLen > 0 {
				if len(got) > tt.maxLen {
					t.Errorf("len %d exceeds max %d: %q", len(got), tt.maxLen, got)
				}
				return
			}
			if got != tt.want {
				t.Errorf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestTask_WorktreePath(t *testing.T) {
	tests := []struct {
		name     string
		task     Task
		wantPath string
	}{
		{
			name:     "slashes become plus",
			task:     Task{RepoPath: "/repos/foo", Branch: "td/my-feature"},
			wantPath: "/repos/foo/.claude/worktrees/td+my-feature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.task.WorktreePath(); got != tt.wantPath {
				t.Errorf("got %q want %q", got, tt.wantPath)
			}
		})
	}
}

func TestTask_Elapsed(t *testing.T) {
	tests := []struct {
		name       string
		task       Task
		minElapsed time.Duration // for running tasks
		wantExact  time.Duration // for finished tasks
	}{
		{
			name: "running uses time since start",
			task: Task{
				Status:    StatusRunning,
				StartedAt: time.Now().Add(-2 * time.Minute),
			},
			minElapsed: time.Minute,
		},
		{
			name: "done uses duration ms",
			task: Task{
				Status:     StatusDone,
				DurationMs: 5000,
			},
			wantExact: 5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.task.Elapsed()
			if tt.wantExact != 0 && got != tt.wantExact {
				t.Errorf("got %v want %v", got, tt.wantExact)
			}
			if tt.minElapsed != 0 && got < tt.minElapsed {
				t.Errorf("got %v want at least %v", got, tt.minElapsed)
			}
		})
	}
}

func TestTaskID(t *testing.T) {
	tests := []struct {
		name     string
		repoA    string
		textA    string
		repoB    string
		textB    string
		wantSame bool
	}{
		{"same repo and text", "repo", "task", "repo", "task", true},
		{"different repo", "repo-a", "task", "repo-b", "task", false},
		{"different text", "repo", "a", "repo", "b", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := TaskID(tt.repoA, tt.textA)
			b := TaskID(tt.repoB, tt.textB)
			if tt.wantSame && a != b {
				t.Errorf("expected same id: %q vs %q", a, b)
			}
			if !tt.wantSame && a == b {
				t.Errorf("expected different ids, both %q", a)
			}
		})
	}
}
