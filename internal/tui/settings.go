package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"

	"td/internal/config"
)

func (m Model) handleSettingsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "tab", "esc":
		if m.confirmDelete {
			m.confirmDelete = false
			return m, nil
		}
		m.viewMode = viewTasks
		return m, nil

	case "j", "down":
		if m.settingsCursor < len(m.cfg.Repos)-1 {
			m.settingsCursor++
			m.confirmDelete = false
		}
		return m, nil

	case "k", "up":
		if m.settingsCursor > 0 {
			m.settingsCursor--
			m.confirmDelete = false
		}
		return m, nil

	case "s":
		m.scanMode = true
		m.scanInput = "~/Development/"
		return m, nil

	case "e":
		m.editingConfig = true
		return m, m.openConfig()

	case "D":
		if len(m.cfg.Repos) == 0 {
			return m, nil
		}
		m.confirmDelete = true
		return m, nil

	case "enter":
		if !m.confirmDelete || len(m.cfg.Repos) == 0 {
			return m, nil
		}
		repo := m.cfg.Repos[m.settingsCursor]
		if err := config.RemoveRepo(repo.Name); err != nil {
			m.err = err
		} else {
			m.cfg.Repos = append(m.cfg.Repos[:m.settingsCursor], m.cfg.Repos[m.settingsCursor+1:]...)
			if m.settingsCursor >= len(m.cfg.Repos) {
				m.settingsCursor = max(0, len(m.cfg.Repos)-1)
			}
			m.status = fmt.Sprintf("Removed %s", repo.Name)
		}
		m.confirmDelete = false
		return m, nil

	case "?":
		m.showHelp = true
		return m, nil
	}

	return m, nil
}

func (m Model) settingsView() string {
	w := m.contentWidth()
	var b strings.Builder

	contentW := w - 4
	header := headerBox.Width(contentW).Render(
		lipgloss.NewStyle().Bold(true).Render("td settings"),
	)
	b.WriteString(header)
	b.WriteString("\n\n")

	repoCount := len(m.cfg.Repos)
	b.WriteString(renderSection(fmt.Sprintf("Repos (%d)", repoCount), w))
	b.WriteString("\n\n")

	if repoCount == 0 {
		b.WriteString(dimStyle.Render("  No repos configured. Press s to scan a directory.") + "\n")
	} else {
		for i, repo := range m.cfg.Repos {
			selected := i == m.settingsCursor
			cursor := "    "
			if selected {
				cursor = cursorStyle.Render(" ›") + " "
			}

			nameStyle := settingsRepoName
			if selected {
				nameStyle = settingsSelectedName
			}

			name := nameStyle.Render(repo.Name)
			path := settingsRepoPath.Render(repo.Path)

			nameWidth := lipgloss.Width(name)
			pathPad := 24 - nameWidth
			if pathPad < 2 {
				pathPad = 2
			}

			b.WriteString(cursor + name + strings.Repeat(" ", pathPad) + path + "\n")

			if repo.Description != "" {
				descPad := strings.Repeat(" ", 4+24)
				b.WriteString(descPad + settingsRepoDesc.Render(repo.Description) + "\n")
			}
		}
	}

	if m.confirmDelete && len(m.cfg.Repos) > 0 {
		repo := m.cfg.Repos[m.settingsCursor]
		b.WriteString("\n")
		b.WriteString(settingsDeleteWarn.Render(
			fmt.Sprintf("  Delete %s? Press Enter to confirm, Esc to cancel.", repo.Name),
		) + "\n")
	}

	mcpCount := len(m.cfg.MCPs)
	b.WriteString("\n")
	b.WriteString(renderSection(fmt.Sprintf("MCP targets (%d)", mcpCount), w))
	b.WriteString("\n\n")

	if mcpCount == 0 {
		b.WriteString(dimStyle.Render("  No MCP targets. Add [[mcp]] blocks in config (e) — requires claude mcp add for each server.") + "\n")
	} else {
		for _, mcp := range m.cfg.MCPs {
			name := settingsRepoName.Render(mcp.Name + " ◈")
			desc := ""
			if mcp.Description != "" {
				desc = settingsRepoDesc.Render(mcp.Description)
			}
			b.WriteString("    " + name)
			if desc != "" {
				b.WriteString("  " + desc)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n  " + dimStyle.Render("Dispatched in generic_workspace with MCP-focused prompts.") + "\n")
	}

	b.WriteString("\n")
	b.WriteString(renderSection("Config", w))
	b.WriteString("\n\n")

	configPath := config.ConfigPath()
	fmt.Fprintf(&b, "  %s %s\n", settingsLabel.Render("path"), settingsValue.Render(configPath))
	fmt.Fprintf(&b, "  %s %s\n", settingsLabel.Render("tasks"), settingsValue.Render(m.cfg.TasksDir))
	fmt.Fprintf(&b, "  %s %s\n", settingsLabel.Render("generic"), settingsValue.Render(m.cfg.GenericWorkspace))
	fmt.Fprintf(&b, "  %s %s\n", settingsLabel.Render("model"), settingsValue.Render(m.cfg.AnthropicModel))

	if m.err != nil {
		b.WriteString("\n" + errorStyle.Render("  "+m.err.Error()) + "\n")
	} else if m.status != "" {
		b.WriteString("\n" + statusStyle.Render("  "+m.status) + "\n")
	}

	b.WriteString("\n")
	footer := []string{
		keyHint("s", "scan"),
		keyHint("e", "edit config"),
		keyHint("D", "delete repo"),
		keyHint("Tab", "back"),
		keyHint("?", "help"),
		keyHint("q", "quit"),
	}
	b.WriteString("  " + strings.Join(footer, "  ") + "\n")

	return b.String()
}
