package tui

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"

	"td/internal/config"
	"td/internal/tracker"
)

const (
	historySidebarWidth = 32
	historyMaxSessions  = 20
	historyMinTermWidth = 72
)

var (
	historyBorderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	historyTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252"))
	historySelected    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	historyMetaStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
)

func (m *Model) refreshHistorySessions() {
	if m.trackerStore == nil {
		m.historySessions = nil
		return
	}
	m.historySessions = m.trackerStore.ListRecent(historyMaxSessions)
	if m.historyCursor >= len(m.historySessions) {
		m.historyCursor = max(0, len(m.historySessions)-1)
	}
}

func (m Model) canShowHistorySidebar() bool {
	return m.width >= historyMinTermWidth
}

func (m Model) tasksPaneWidth() int {
	w := m.innerWidth()
	if m.showHistory && m.canShowHistorySidebar() {
		return w - historySidebarWidth - 1
	}
	return w
}

func (m Model) selectedHistorySession() *tracker.Task {
	if m.historyCursor < 0 || m.historyCursor >= len(m.historySessions) {
		return nil
	}
	return m.historySessions[m.historyCursor]
}

func (m Model) handleHistoryKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "h", "esc":
		m.showHistory = false
		m.confirmHistoryDel = false
		m.pendingResumeID = ""
		return m, nil

	case "?":
		m.showHelp = true
		return m, nil

	case "tab":
		m.showHistory = false
		m.confirmHistoryDel = false
		m.pendingResumeID = ""
		m.viewMode = viewSettings
		m.confirmDelete = false
		return m, nil

	case "j", "down":
		if m.historyCursor < len(m.historySessions)-1 {
			m.historyCursor++
		}
		m.confirmHistoryDel = false
		m.pendingResumeID = ""

	case "k", "up":
		if m.historyCursor > 0 {
			m.historyCursor--
		}
		m.confirmHistoryDel = false
		m.pendingResumeID = ""

	case "g":
		m.historyCursor = 0
		m.confirmHistoryDel = false
		m.pendingResumeID = ""

	case "G":
		if len(m.historySessions) > 0 {
			m.historyCursor = len(m.historySessions) - 1
		}
		m.confirmHistoryDel = false
		m.pendingResumeID = ""

	case "enter", "r":
		tr := m.selectedHistorySession()
		if tr != nil && tr.Status == tracker.StatusRunning {
			if m.pendingResumeID == tr.ID {
				// Second press confirmed — proceed with resume.
				m.pendingResumeID = ""
				return m.resumeTrackerSession(tr)
			}
			// First press — require confirmation.
			m.pendingResumeID = tr.ID
			return m, nil
		}
		m.pendingResumeID = ""
		return m.resumeTrackerSession(tr)

	case "w":
		m.pendingResumeID = ""
		tr := m.selectedHistorySession()
		if tr == nil || m.trackerStore == nil {
			return m, nil
		}
		logPath := m.trackerStore.LogPath(tr.ID)
		m.watchMode = true
		m.watchModel = newWatchModel(tr, logPath, m.width, m.height)
		return m, m.watchModel.init()

	case "l", "L":
		return m.openHistoryLog()

	case "D", "ctrl+d":
		tr := m.selectedHistorySession()
		if tr == nil {
			return m, nil
		}
		if m.confirmHistoryDel {
			if m.trackerStore != nil {
				_ = m.trackerStore.Remove(tr.ID)
			}
			m.refreshHistorySessions()
			m.status = "Session removed"
			m.confirmHistoryDel = false
			return m, nil
		}
		m.confirmHistoryDel = true
		m.pendingResumeID = ""
		return m, nil

	default:
		return m, nil
	}

	return m, nil
}

func (m Model) openHistoryLog() (tea.Model, tea.Cmd) {
	tr := m.selectedHistorySession()
	if tr == nil || m.trackerStore == nil {
		return m, nil
	}
	logPath := m.trackerStore.LogPath(tr.ID)
	c := exec.Command(m.cfg.Editor, logPath)
	return m, tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{err: err}
	})
}

func (m Model) renderHistorySidebar() string {
	w := historySidebarWidth
	var lines []string

	lines = append(lines, historyTitleStyle.Render("Sessions"))
	lines = append(lines, historyBorderStyle.Render(strings.Repeat("─", w-2)))

	if len(m.historySessions) == 0 {
		lines = append(lines, "")
		lines = append(lines, dimStyle.Render("  No sessions yet."))
		lines = append(lines, dimStyle.Render("  Dispatch a task"))
		lines = append(lines, dimStyle.Render("  with d."))
	} else {
		for i, tr := range m.historySessions {
			lines = append(lines, m.renderHistoryRow(i, tr, w))
		}
	}

	if m.pendingResumeID != "" {
		lines = append(lines, "")
		lines = append(lines, settingsDeleteWarn.Render("  ⚠ Session is running."))
		lines = append(lines, settingsDeleteWarn.Render("  r again to confirm,"))
		lines = append(lines, dimStyle.Render("  or w to watch."))
	} else if m.confirmHistoryDel {
		lines = append(lines, "")
		lines = append(lines, settingsDeleteWarn.Render("  Delete session? D again"))
	}

	lines = append(lines, "")
	lines = append(lines, dimStyle.Render("  Enter/r resume  w watch"))
	lines = append(lines, dimStyle.Render("  L log  D remove"))
	lines = append(lines, dimStyle.Render("  h hide"))

	content := strings.Join(lines, "\n")
	border := historyBorderStyle.Render("│")
	padded := lipgloss.NewStyle().Padding(0, 0, 0, 1).Render(content)
	return lipgloss.JoinHorizontal(lipgloss.Top, border, padded)
}

func (m Model) renderHistoryRow(idx int, tr *tracker.Task, w int) string {
	selected := idx == m.historyCursor
	title := sessionTitle(tr.TaskText, w-4)
	if selected {
		title = historySelected.Render(truncate(title, w-4))
	} else {
		title = truncate(title, w-4)
	}

	repo := tr.RepoName
	switch {
	case repo == config.GenericRepoName:
		repo = "generic ✦"
	case m.cfg.IsMCP(repo):
		repo = repo + " ◈"
	}
	meta := fmt.Sprintf("%s · %s", repo, sessionStatusShort(tr.Status))
	if tr.Status == tracker.StatusRunning || tr.Status == tracker.StatusNeedsInput {
		meta += " · " + formatRelativeStart(tr.StartedAt)
	} else if tr.CostUSD > 0 {
		meta += fmt.Sprintf(" · $%.2f", tr.CostUSD)
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + title + "\n")
	if selected {
		b.WriteString("  " + historyMetaStyle.Render(truncate(meta, w-2)))
	} else {
		b.WriteString("  " + dimStyle.Render(truncate(meta, w-2)))
	}
	return b.String()
}

func sessionTitle(taskText string, max int) string {
	line := taskText
	if idx := strings.IndexByte(line, '\n'); idx >= 0 {
		line = line[:idx]
	}
	line = strings.TrimSpace(line)
	if max > 0 && len(line) > max {
		return line[:max-1] + "…"
	}
	return line
}

func sessionStatusShort(s tracker.Status) string {
	switch s {
	case tracker.StatusRunning:
		return "running"
	case tracker.StatusDone:
		return "done"
	case tracker.StatusFailed:
		return "failed"
	case tracker.StatusCrashed:
		return "crashed"
	case tracker.StatusNeedsInput:
		return "needs input"
	default:
		return string(s)
	}
}

func formatRelativeStart(t time.Time) string {
	d := time.Since(t)
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	if d < 48*time.Hour {
		return "yesterday"
	}
	return t.Format("Jan 2")
}
