package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFilterNew(t *testing.T) {
	existing := map[string]bool{"alpha": true, "taken": true}

	tests := []struct {
		name     string
		in       []Suggestion
		wantLen  int
		wantNames []string
	}{
		{
			name: "drops existing only",
			in: []Suggestion{
				{Name: "alpha"},
				{Name: "beta"},
			},
			wantLen:   1,
			wantNames: []string{"beta"},
		},
		{
			name:      "all new",
			in:        []Suggestion{{Name: "one"}, {Name: "two"}},
			wantLen:   2,
			wantNames: []string{"one", "two"},
		},
		{
			name:      "all filtered",
			in:        []Suggestion{{Name: "alpha"}, {Name: "taken"}},
			wantLen:   0,
			wantNames: nil,
		},
		{
			name:      "empty input",
			in:        nil,
			wantLen:   0,
			wantNames: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterNew(tt.in, existing)
			if len(got) != tt.wantLen {
				t.Fatalf("len: got %d want %d (%+v)", len(got), tt.wantLen, got)
			}
			for i, name := range tt.wantNames {
				if got[i].Name != name {
					t.Errorf("[%d]: got %q want %q", i, got[i].Name, name)
				}
			}
		})
	}
}

func TestFindRepos(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(root string) error
		wantBases []string
	}{
		{
			name: "finds git repos",
			setup: func(root string) error {
				for _, rel := range []string{"project-a", filepath.Join("nested", "project-b")} {
					p := filepath.Join(root, rel)
					if err := os.MkdirAll(filepath.Join(p, ".git"), 0755); err != nil {
						return err
					}
				}
				return nil
			},
			wantBases: []string{"project-a", "project-b"},
		},
		{
			name: "skips node_modules",
			setup: func(root string) error {
				p := filepath.Join(root, "app")
				if err := os.MkdirAll(filepath.Join(p, ".git"), 0755); err != nil {
					return err
				}
				return os.MkdirAll(filepath.Join(p, "node_modules", ".git"), 0755)
			},
			wantBases: []string{"app"},
		},
		{
			name:      "empty directory",
			setup:     func(string) error { return nil },
			wantBases: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			if err := tt.setup(root); err != nil {
				t.Fatal(err)
			}

			repos, err := FindRepos(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(repos) != len(tt.wantBases) {
				t.Fatalf("count: got %d want %d (%v)", len(repos), len(tt.wantBases), repos)
			}

			found := map[string]bool{}
			for _, r := range repos {
				found[filepath.Base(r)] = true
			}
			for _, base := range tt.wantBases {
				if !found[base] {
					t.Errorf("missing repo %q in %v", base, repos)
				}
			}
		})
	}
}
