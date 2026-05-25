package tracker

import (
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Status string

const (
	StatusRunning    Status = "running"
	StatusDone       Status = "done"
	StatusFailed     Status = "failed"
	StatusCrashed    Status = "crashed"
	StatusNeedsInput Status = "needs_input"
)

type Activity struct {
	Time   time.Time `json:"time"`
	Tool   string    `json:"tool"`
	Detail string    `json:"detail"`
}

type Task struct {
	ID         string    `json:"id"`
	TaskText   string    `json:"task_text"`
	RepoName   string    `json:"repo_name"`
	RepoPath   string    `json:"repo_path"`
	Branch     string    `json:"branch"`
	PID        int       `json:"pid"`
	SessionID  string    `json:"session_id"`
	StartedAt  time.Time `json:"started_at"`
	Status     Status    `json:"status"`
	LastTool   string    `json:"last_tool,omitempty"`
	LastDetail string    `json:"last_detail,omitempty"`
	Turns      int       `json:"turns,omitempty"`
	CostUSD    float64   `json:"cost_usd,omitempty"`
	DurationMs int64     `json:"duration_ms,omitempty"`
	Error      string    `json:"error,omitempty"`
}

func (t Task) Elapsed() time.Duration {
	if t.Status == StatusRunning {
		return time.Since(t.StartedAt)
	}
	return time.Duration(t.DurationMs) * time.Millisecond
}

// WorktreePath returns the path Claude uses for the git worktree.
func (t Task) WorktreePath() string {
	worktreeName := strings.ReplaceAll(t.Branch, "/", "+")
	return filepath.Join(t.RepoPath, ".claude", "worktrees", worktreeName)
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// BranchName generates a branch name like "td/create-snowflake-task".
func BranchName(taskText string) string {
	s := strings.ToLower(taskText)
	s = nonAlnum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 60 {
		s = s[:60]
		s = strings.TrimRight(s, "-")
	}
	return "td/" + s
}
