package taskfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []Task
	}{
		{
			name: "basic tasks and sections",
			content: `## Tasks
- [ ] Plain task
- [x] Done task
- [ ] @agent:my-repo Fix the bug
- [ ] @draft Sketch the API`,
			want: []Task{
				{Text: "Plain task", Status: Open, Section: "Tasks"},
				{Text: "", Status: Done},
				{Text: "Fix the bug", Status: Open, Tag: TagAgent, Target: "my-repo"},
				{Text: "Sketch the API", Status: Open, Tag: TagDraft},
			},
		},
		{
			name: "multiline description",
			content: `## Tasks
- [ ] @agent:repo Title line
  First detail line
  Second detail line`,
			want: []Task{
				{
					Text:        "Title line",
					Status:      Open,
					Tag:         TagAgent,
					Target:      "repo",
					Description: "First detail line\nSecond detail line",
					EndLine:     3,
				},
			},
		},
		{
			name: "tag variants",
			content: `## Tasks
- [ ] @agent:foo bar baz
- [ ] @draft:bar only target
- [ ] @dispatched already sent
- [ ] @agent inline no colon`,
			want: []Task{
				{Text: "bar baz", Tag: TagAgent, Target: "foo"},
				{Text: "only target", Tag: TagDraft, Target: "bar"},
				{Text: "already sent", Tag: TagDispatched},
				{Text: "inline no colon", Tag: TagAgent},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTemp(t, tt.content)
			got, err := Parse(path)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("task count: got %d want %d", len(got), len(tt.want))
			}
			for i, w := range tt.want {
				assertTask(t, i, got[i], w)
			}
		})
	}
}

func assertTask(t *testing.T, i int, got, want Task) {
	t.Helper()
	if want.Text != "" && got.Text != want.Text {
		t.Errorf("task[%d].Text: got %q want %q", i, got.Text, want.Text)
	}
	if want.Status != 0 && got.Status != want.Status {
		t.Errorf("task[%d].Status: got %v want %v", i, got.Status, want.Status)
	}
	if want.Tag != "" && got.Tag != want.Tag {
		t.Errorf("task[%d].Tag: got %v want %v", i, got.Tag, want.Tag)
	}
	if want.Target != "" && got.Target != want.Target {
		t.Errorf("task[%d].Target: got %q want %q", i, got.Target, want.Target)
	}
	if want.Section != "" && got.Section != want.Section {
		t.Errorf("task[%d].Section: got %q want %q", i, got.Section, want.Section)
	}
	if want.Description != "" && got.Description != want.Description {
		t.Errorf("task[%d].Description: got %q want %q", i, got.Description, want.Description)
	}
	if want.EndLine != 0 && got.EndLine != want.EndLine {
		t.Errorf("task[%d].EndLine: got %d want %d", i, got.EndLine, want.EndLine)
	}
}

func TestTask_FullText(t *testing.T) {
	tests := []struct {
		name string
		task Task
		want string
	}{
		{"title only", Task{Text: "Only"}, "Only"},
		{"title and description", Task{Text: "Title", Description: "Detail"}, "Title\nDetail"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.task.FullText(); got != tt.want {
				t.Errorf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestUpdateTask(t *testing.T) {
	path := writeTemp(t, `## Tasks
- [ ] Old title
  old detail
`)

	tasks, err := Parse(path)
	if err != nil {
		t.Fatal(err)
	}

	updates := []struct {
		name string
		task Task
	}{
		{
			name: "mark done with agent tag",
			task: Task{
				Line:        tasks[0].Line,
				EndLine:     tasks[0].EndLine,
				Status:      Done,
				Tag:         TagAgent,
				Target:      "repo",
				Text:        "New title",
				Description: "new detail",
			},
		},
	}

	for _, tt := range updates {
		t.Run(tt.name, func(t *testing.T) {
			if err := UpdateTask(path, tt.task); err != nil {
				t.Fatal(err)
			}
			got, err := Parse(path)
			if err != nil {
				t.Fatal(err)
			}
			assertTask(t, 0, got[0], tt.task)
		})
	}
}

func TestDeleteTask(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		deleteIndex    int
		mustContain    []string
		mustNotContain []string
	}{
		{
			name: "removes task and description lines",
			content: `## Tasks
- [ ] Keep me
- [ ] Remove me
  with detail
## Notes
note`,
			deleteIndex:    1,
			mustContain:    []string{"Keep me", "## Notes"},
			mustNotContain: []string{"Remove me", "with detail"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTemp(t, tt.content)
			tasks, err := Parse(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := DeleteTask(path, tasks[tt.deleteIndex]); err != nil {
				t.Fatal(err)
			}

			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			content := string(data)
			for _, s := range tt.mustContain {
				if !strings.Contains(content, s) {
					t.Errorf("missing %q in:\n%s", s, content)
				}
			}
			for _, s := range tt.mustNotContain {
				if strings.Contains(content, s) {
					t.Errorf("still contains %q in:\n%s", s, content)
				}
			}
		})
	}
}

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}
