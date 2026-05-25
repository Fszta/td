package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type Repo struct {
	Name        string `toml:"name"`
	Path        string `toml:"path"`
	Description string `toml:"description"`
}

// MCP describes an external tool integration (Notion, Linear, etc.) that
// headless Claude can use when MCP servers are configured for Claude Code.
type MCP struct {
	Name        string `toml:"name"`
	Description string `toml:"description"`
}

type Config struct {
	TasksDir         string `toml:"tasks_dir"`
	Editor           string `toml:"editor"`
	AnthropicAPIKey  string `toml:"anthropic_api_key"`
	AnthropicModel   string `toml:"anthropic_model"`
	GenericWorkspace string `toml:"generic_workspace"`
	Repos            []Repo `toml:"repos"`
	MCPs             []MCP  `toml:"mcp"`
}

// GenericRepoName is the reserved target name used when an actionable task
// has no configured repo home. Dispatches with this target run in
// Config.GenericWorkspace instead of a real repo path.
const GenericRepoName = "generic"

func Load() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home dir: %w", err)
	}

	cfg := &Config{
		TasksDir:         filepath.Join(home, "tasks"),
		Editor:           "nvim",
		AnthropicModel:   "claude-sonnet-4-6",
		GenericWorkspace: filepath.Join(home, "scratchpad", "td"),
	}

	configDir, err := os.UserConfigDir()
	if err == nil {
		configPath := filepath.Join(configDir, "td", "config.toml")
		if _, statErr := os.Stat(configPath); statErr == nil {
			if _, decErr := toml.DecodeFile(configPath, cfg); decErr != nil {
				return nil, fmt.Errorf("parsing %s: %w", configPath, decErr)
			}
		}
	}

	cfg.TasksDir = expandHome(cfg.TasksDir)
	cfg.GenericWorkspace = expandHome(cfg.GenericWorkspace)
	for i := range cfg.Repos {
		cfg.Repos[i].Path = expandHome(cfg.Repos[i].Path)
	}

	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		cfg.AnthropicAPIKey = key
	}

	return cfg, nil
}

const configTemplate = `# td configuration

# tasks_dir = "~/tasks"
# editor = "nvim"
# anthropic_api_key = "sk-ant-..."
# anthropic_model = "claude-sonnet-4-6"

# Workspace for tasks dispatched with @agent:generic (drafts, SQL files,
# one-off scripts, new project bootstraps...). Defaults to ~/scratchpad/td.
# generic_workspace = "~/scratchpad/td"

# Add repositories for task dispatch.
# Run 'td scan <directory>' to auto-discover repos.
#
# [[repos]]
# name = "my-repo"
# path = "~/Development/my-repo"
# description = "What this repo does"
#
# MCP targets for tasks that should run with external tools (requires the same
# MCP servers configured in Claude Code: claude mcp list).
#
# [[mcp]]
# name = "notion"
# description = "Create and update Notion pages and databases"
#
# [[mcp]]
# name = "linear"
# description = "Create and update Linear issues and projects"
`

// FindMCP returns a configured MCP by name (case-insensitive).
func (c *Config) FindMCP(name string) *MCP {
	for i := range c.MCPs {
		if strings.EqualFold(c.MCPs[i].Name, name) {
			return &c.MCPs[i]
		}
	}
	return nil
}

// IsMCP reports whether name is a configured MCP target (not a repo).
func (c *Config) IsMCP(name string) bool {
	return c.FindMCP(name) != nil
}

func EnsureConfigFile() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("getting config dir: %w", err)
	}

	dir := filepath.Join(configDir, "td")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	configPath := filepath.Join(dir, "config.toml")
	if _, err := os.Stat(configPath); err == nil {
		return configPath, nil
	}

	if err := os.WriteFile(configPath, []byte(configTemplate), 0644); err != nil {
		return "", err
	}
	return configPath, nil
}

func ConfigPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(configDir, "td", "config.toml")
}

func AppendRepos(repos []Repo) error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("getting config dir: %w", err)
	}

	dir := filepath.Join(configDir, "td")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	configPath := filepath.Join(dir, "config.toml")
	f, err := os.OpenFile(configPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, r := range repos {
		fmt.Fprintf(f, "\n[[repos]]\nname = %q\npath = %q\ndescription = %q\n", r.Name, r.Path, r.Description)
	}
	return nil
}

func ExistingRepoNames(cfg *Config) map[string]bool {
	names := make(map[string]bool, len(cfg.Repos))
	for _, r := range cfg.Repos {
		names[r.Name] = true
	}
	return names
}

func RemoveRepo(name string) error {
	configPath := ConfigPath()
	if configPath == "" {
		return fmt.Errorf("config path not found")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	var result []string
	i := 0

	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])

		if trimmed == "[[repos]]" {
			blockStart := i
			blockEnd := i + 1
			isTarget := false

			for blockEnd < len(lines) {
				bl := strings.TrimSpace(lines[blockEnd])
				if bl == "" || strings.HasPrefix(bl, "[[") || strings.HasPrefix(bl, "[") {
					break
				}
				if strings.HasPrefix(bl, "name") {
					parts := strings.SplitN(bl, "=", 2)
					if len(parts) == 2 {
						val := strings.TrimSpace(parts[1])
						val = strings.Trim(val, "\"'")
						if strings.EqualFold(val, name) {
							isTarget = true
						}
					}
				}
				blockEnd++
			}

			if isTarget {
				i = blockEnd
				for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
					i++
				}
				continue
			}

			for j := blockStart; j < blockEnd; j++ {
				result = append(result, lines[j])
			}
			i = blockEnd
			continue
		}

		result = append(result, lines[i])
		i++
	}

	return os.WriteFile(configPath, []byte(strings.Join(result, "\n")), 0644)
}

func expandHome(path string) string {
	if path == "~" {
		home, _ := os.UserHomeDir()
		return home
	}
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}
