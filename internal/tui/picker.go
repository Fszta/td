package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"

	"td/internal/config"
	"td/internal/taskfile"
)

func (m Model) handlePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		filtered := m.filteredRepos()
		if len(filtered) == 0 || m.pickerCursor >= len(filtered) {
			return m, nil
		}
		repo := filtered[m.pickerCursor]
		t := m.currentTask()
		if t == nil {
			return m, nil
		}
		if t.Tag != taskfile.TagAgent && t.Tag != taskfile.TagDraft {
			t.Tag = taskfile.TagAgent
		}
		t.Target = repo.Name
		if err := taskfile.UpdateTask(m.filePath, *t); err != nil {
			m.err = err
		}
		forDispatch := m.pickerForDispatch
		m.pickerInput = ""
		m.pickerCursor = 0
		m.pickerForDispatch = false
		if forDispatch {
			cfgRepo := m.findRepo(repo.Name)
			if cfgRepo != nil {
				m.dispatchTarget = cfgRepo
				m.dispatch = dstateConfirm
				return m, nil
			}
		}
		m.dispatch = dstateNone
		return m, nil

	case "esc":
		m.dispatch = dstateNone
		m.pickerInput = ""
		m.pickerCursor = 0
		return m, nil

	case "ctrl+c":
		return m, tea.Quit

	case "backspace":
		if len(m.pickerInput) > 0 {
			m.pickerInput = m.pickerInput[:len(m.pickerInput)-1]
			m.pickerCursor = 0
		}

	case "up", "ctrl+p":
		if m.pickerCursor > 0 {
			m.pickerCursor--
		}

	case "down", "ctrl+n":
		filtered := m.filteredRepos()
		if m.pickerCursor < len(filtered)-1 {
			m.pickerCursor++
		}

	default:
		r := msg.String()
		if len(r) == 1 && r[0] >= 32 && r[0] < 127 {
			m.pickerInput += r
			m.pickerCursor = 0
		}
	}

	return m, nil
}

func (m Model) filteredRepos() []config.Repo {
	generic := *m.genericRepo()
	all := make([]config.Repo, 0, len(m.cfg.Repos)+len(m.cfg.MCPs)+1)
	all = append(all, generic)
	for i := range m.cfg.MCPs {
		all = append(all, *m.mcpRepo(&m.cfg.MCPs[i]))
	}
	all = append(all, m.cfg.Repos...)

	if m.pickerInput == "" {
		return all
	}
	input := strings.ToLower(m.pickerInput)
	var filtered []config.Repo
	for _, r := range all {
		if strings.Contains(strings.ToLower(r.Name), input) ||
			strings.Contains(strings.ToLower(r.Description), input) {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func (m Model) pickerView() string {
	w := m.contentWidth()
	var b strings.Builder

	contentW := w - 4
	header := headerBox.Width(contentW).Render(
		lipgloss.NewStyle().Bold(true).Render("assign target"),
	)
	b.WriteString(header)
	b.WriteString("\n\n")

	inputLine := m.pickerInput
	if inputLine == "" {
		inputLine = dimStyle.Render("type to filter...")
	}
	fmt.Fprintf(&b, "  > %s\n\n", inputLine)

	filtered := m.filteredRepos()
	for i, r := range filtered {
		cursor := "    "
		if i == m.pickerCursor {
			cursor = cursorStyle.Render(" ›") + " "
		}

		name := r.Name
		if m.cfg.IsMCP(r.Name) {
			name = r.Name + " ◈"
		} else if r.Name == config.GenericRepoName {
			name = r.Name + " ✦"
		}
		if i == m.pickerCursor {
			name = selectedText.Render(name)
		} else if m.cfg.IsMCP(r.Name) {
			name = badgeAgentMCP.Render(name)
		} else if r.Name == config.GenericRepoName {
			name = badgeAgentGen.Render(name)
		} else {
			name = settingsRepoName.Render(name)
		}

		desc := ""
		if r.Description != "" {
			desc = dimStyle.Render(r.Description)
		}

		nameWidth := lipgloss.Width(name)
		pad := max(24-nameWidth, 2)

		b.WriteString(cursor + name + strings.Repeat(" ", pad) + desc + "\n")
	}

	if len(filtered) == 0 {
		b.WriteString(dimStyle.Render("  no matching targets") + "\n")
	}

	b.WriteString("\n  " + keyHint("Enter", "select") + "  " + keyHint("Esc", "cancel") + "\n")
	return b.String()
}
