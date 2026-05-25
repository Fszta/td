package tracker

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"
)

type Store struct {
	dir string
}

type state struct {
	Tasks map[string]*Task `json:"tasks"`
}

func NewStore() (*Store, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(configDir, "td", "trackers")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

func (s *Store) statePath() string {
	return filepath.Join(s.dir, "state.json")
}

func (s *Store) LogPath(id string) string {
	return filepath.Join(s.dir, id+".jsonl")
}

func TaskID(repoName, taskText string) string {
	h := sha256.Sum256([]byte(repoName + ":" + taskText))
	return fmt.Sprintf("%x", h[:8])
}

func (s *Store) load() *state {
	st := &state{Tasks: make(map[string]*Task)}
	data, err := os.ReadFile(s.statePath())
	if err != nil {
		return st
	}
	_ = json.Unmarshal(data, st)
	if st.Tasks == nil {
		st.Tasks = make(map[string]*Task)
	}
	return st
}

func (s *Store) save(st *state) error {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.statePath(), data, 0644)
}

func (s *Store) Register(t *Task) error {
	st := s.load()
	st.Tasks[t.ID] = t
	return s.save(st)
}

func (s *Store) Update(t *Task) error {
	return s.Register(t)
}

func (s *Store) Get(id string) *Task {
	st := s.load()
	return st.Tasks[id]
}

func (s *Store) ActiveTasks() []*Task {
	st := s.load()
	var active []*Task
	for _, t := range st.Tasks {
		if t.Status == StatusRunning {
			active = append(active, t)
		}
	}
	return active
}

func (s *Store) AllTasks() map[string]*Task {
	return s.load().Tasks
}

// ListRecent returns tasks sorted by StartedAt (newest first), capped at limit.
// limit <= 0 means no cap.
func (s *Store) ListRecent(limit int) []*Task {
	st := s.load()
	all := make([]*Task, 0, len(st.Tasks))
	for _, t := range st.Tasks {
		all = append(all, t)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].StartedAt.After(all[j].StartedAt)
	})
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all
}

// Remove deletes a tracker and its log file.
func (s *Store) Remove(id string) error {
	st := s.load()
	delete(st.Tasks, id)
	os.Remove(s.LogPath(id))
	return s.save(st)
}

// IsProcessAlive checks if a PID is still running.
func IsProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
