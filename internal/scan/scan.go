package scan

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"td/internal/ai"
	"td/internal/config"
)

type Suggestion struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Description string `json:"description"`
	Selected    bool   `json:"-"`
}

func FindRepos(dir string) ([]string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	var repos []string
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		switch d.Name() {
		case ".git", "node_modules", "vendor", ".terraform", "__pycache__", ".venv":
			return filepath.SkipDir
		}
		gitDir := filepath.Join(path, ".git")
		if info, statErr := os.Stat(gitDir); statErr == nil && info.IsDir() {
			repos = append(repos, path)
		}
		return nil
	})

	return repos, nil
}

const scanSystemPrompt = `You are a repository analyzer. Given information about git repositories, generate a concise name and one-line description for each.

Rules:
- name: lowercase, hyphenated, concise (e.g., "webhook-gateway", "terraform-ecr", "analytics")
- description: one sentence describing what the repo does and its main tech
- Return ONLY a valid JSON array, no explanation or code fences

Output format:
[{"path": "/full/path", "name": "repo-name", "description": "What this repo does"}]`

func DescribeRepos(paths []string, apiKey, model string) ([]Suggestion, error) {
	var prompt strings.Builder
	prompt.WriteString(fmt.Sprintf("Analyze these %d git repositories:\n\n", len(paths)))

	for i, p := range paths {
		ctx := collectContext(p)
		fmt.Fprintf(&prompt, "--- Repository %d ---\n%s\n", i+1, ctx)
	}

	client := ai.NewClient(apiKey, model)
	result, err := client.Complete(scanSystemPrompt, prompt.String())
	if err != nil {
		return nil, err
	}

	result = strings.TrimSpace(result)
	if strings.HasPrefix(result, "```") {
		lines := strings.Split(result, "\n")
		if len(lines) > 2 {
			result = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}

	var raw []Suggestion
	if err := json.Unmarshal([]byte(result), &raw); err != nil {
		return nil, fmt.Errorf("parsing AI response: %w\nraw: %s", err, result)
	}

	for i := range raw {
		raw[i].Selected = true
	}
	return raw, nil
}

func FilterNew(suggestions []Suggestion, existing map[string]bool) []Suggestion {
	var filtered []Suggestion
	for _, s := range suggestions {
		if !existing[s.Name] {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

func SaveSelected(suggestions []Suggestion) error {
	var repos []config.Repo
	for _, s := range suggestions {
		if s.Selected {
			home, _ := os.UserHomeDir()
			path := s.Path
			if strings.HasPrefix(path, home) {
				path = "~" + path[len(home):]
			}
			repos = append(repos, config.Repo{
				Name:        s.Name,
				Path:        path,
				Description: s.Description,
			})
		}
	}
	if len(repos) == 0 {
		return nil
	}
	return config.AppendRepos(repos)
}

func collectContext(repoPath string) string {
	var ctx strings.Builder
	ctx.WriteString(fmt.Sprintf("Path: %s\n", repoPath))

	entries, err := os.ReadDir(repoPath)
	if err == nil {
		var names []string
		for _, e := range entries {
			if !strings.HasPrefix(e.Name(), ".") {
				names = append(names, e.Name())
			}
		}
		ctx.WriteString(fmt.Sprintf("Files: %s\n", strings.Join(names, ", ")))
	}

	for _, readme := range []string{"README.md", "readme.md", "README"} {
		data, err := os.ReadFile(filepath.Join(repoPath, readme))
		if err == nil {
			content := string(data)
			if len(content) > 500 {
				content = content[:500] + "..."
			}
			ctx.WriteString(fmt.Sprintf("README:\n%s\n", content))
			break
		}
	}

	for _, cfgFile := range []string{"go.mod", "pyproject.toml", "package.json", "Cargo.toml"} {
		data, err := os.ReadFile(filepath.Join(repoPath, cfgFile))
		if err == nil {
			content := string(data)
			if len(content) > 300 {
				content = content[:300] + "..."
			}
			ctx.WriteString(fmt.Sprintf("%s:\n%s\n", cfgFile, content))
			break
		}
	}

	return ctx.String()
}
