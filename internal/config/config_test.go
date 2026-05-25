package config

import (
	"testing"
)

func TestFindMCP(t *testing.T) {
	cfg := &Config{
		MCPs: []MCP{
			{Name: "notion", Description: "Notion pages"},
			{Name: "Linear", Description: "Issues"},
		},
	}

	tests := []struct {
		name     string
		query    string
		wantName string // empty = expect nil
	}{
		{"case insensitive match", "NOTION", "notion"},
		{"mixed case", "linear", "Linear"},
		{"unknown", "jira", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.FindMCP(tt.query)
			if tt.wantName == "" {
				if got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil || got.Name != tt.wantName {
				t.Errorf("FindMCP(%q): got %+v want name %q", tt.query, got, tt.wantName)
			}
		})
	}
}

func TestIsMCP(t *testing.T) {
	cfg := &Config{
		MCPs: []MCP{{Name: "notion"}},
	}

	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"configured", "notion", true},
		{"case insensitive", "NOTION", true},
		{"missing", "linear", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cfg.IsMCP(tt.in); got != tt.want {
				t.Errorf("IsMCP(%q): got %v want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestExistingRepoNames(t *testing.T) {
	cfg := &Config{
		Repos: []Repo{
			{Name: "alpha"},
			{Name: "beta"},
		},
	}

	tests := []struct {
		name  string
		repo  string
		want  bool
	}{
		{"alpha exists", "alpha", true},
		{"beta exists", "beta", true},
		{"gamma missing", "gamma", false},
	}

	names := ExistingRepoNames(cfg)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := names[tt.repo]; got != tt.want {
				t.Errorf("names[%q]: got %v want %v", tt.repo, got, tt.want)
			}
		})
	}
}

func TestGenericRepoName(t *testing.T) {
	if GenericRepoName != "generic" {
		t.Errorf("got %q want generic", GenericRepoName)
	}
}
