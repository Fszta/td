package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"td/internal/config"
	"td/internal/taskfile"
	"td/internal/tracker"
)

func (m Model) View() string {
	if m.showHelp {
		return m.helpView()
	}
	if m.watchMode {
		return m.watchModel.view()
	}
	if m.loading {
		return m.loadingView()
	}
	if m.dispatch == dstateConfirm {
		return m.dispatchConfirmView()
	}
	if m.dispatch == dstatePicker {
		return m.pickerView()
	}
	if m.scanMode {
		return m.scanInputView()
	}
	if m.viewMode == viewSettings {
		return m.settingsView()
	}
	return m.mainView()
}

func (m Model) mainView() string {
	w := m.tasksPaneWidth()

	var lines []string

	lines = append(lines, m.renderTitle(w))
	done, total := m.taskCounts()
	lines = append(lines, renderProgressBar(done, total, w))
	lines = append(lines, m.renderCounters(w))
	lines = append(lines, "")

	if len(m.tasks) == 0 {
		lines = append(lines, dimStyle.Render("No tasks yet. Here's how to get started:"))
		lines = append(lines, "")
		lines = append(lines, footerKey.Render("e")+footerDesc.Render("  open editor — write tasks as free text"))
		lines = append(lines, footerKey.Render("f")+footerDesc.Render("  format — Claude structures and classifies"))
		lines = append(lines, footerKey.Render("d")+footerDesc.Render("  dispatch — send agent tasks to repo, MCP, or generic"))
		lines = append(lines, "")
		lines = append(lines, dimStyle.Render("Press e to begin."))
	} else {
		lastCat := taskCategory(-1)
		for pos, realIdx := range m.displayOrder {
			task := m.tasks[realIdx]
			cat := taskCat(task)
			if cat != lastCat {
				if lastCat >= 0 {
					lines = append(lines, "")
				}
				lines = append(lines, renderCategoryHeader(cat, w))
				lines = append(lines, "")
				lastCat = cat
			}
			if cat == catDone {
				lines = append(lines, m.renderDoneTask(pos, task, w))
			} else {
				lines = append(lines, m.renderTask(pos, realIdx, task, w))
			}
		}

		c := m.counts()
		if c.done > 0 && !m.doneExpanded {
			lines = append(lines, "")
			lines = append(lines, catDoneSummary.Render(
				fmt.Sprintf("  ✓ %d completed  ", c.done))+
				dimStyle.Render("(v to expand)"),
			)
		}
	}

	if m.confirmTaskDel && m.cursor < len(m.displayOrder) {
		t := m.currentTask()
		lines = append(lines, settingsDeleteWarn.Render(
			fmt.Sprintf("Delete \"%s\"? Press D again.", truncate(t.Text, w-30)),
		))
	} else if m.err != nil {
		lines = append(lines, errorStyle.Render(m.err.Error()))
	} else if m.status != "" {
		lines = append(lines, statusStyle.Render(m.status))
	}

	lines = append(lines, "")
	lines = append(lines, separatorStyle.Render(strings.Repeat("─", w)))
	lines = append(lines, m.renderFooter())

	main := frameStyle.Width(w).Render(strings.Join(lines, "\n"))
	if m.showHistory && m.canShowHistorySidebar() {
		return lipgloss.JoinHorizontal(lipgloss.Top, main, m.renderHistorySidebar())
	}
	return main
}

func (m Model) renderTitle(w int) string {
	date := time.Now().Format("Monday, Jan 2 2006")
	title := lipgloss.NewStyle().Bold(true).Render("td")
	fileName := strings.TrimSuffix(filepath.Base(m.filePath), ".md")
	left := title + " " + fileNameStyle.Render("· "+fileName)

	dateRendered := dimStyle.Render(date)
	pad := w - lipgloss.Width(left) - lipgloss.Width(dateRendered)
	if pad < 1 {
		pad = 1
	}
	return left + strings.Repeat(" ", pad) + dateRendered
}

func renderProgressBar(done, total, width int) string {
	if total == 0 {
		return progressLabel.Render("no tasks")
	}

	label := fmt.Sprintf(" %d/%d done", done, total)
	labelWidth := len(label)
	barWidth := width - labelWidth
	if barWidth > 35 {
		barWidth = 35
	}
	if barWidth < 10 {
		barWidth = 10
	}

	filledWidth := barWidth * done / total
	emptyWidth := barWidth - filledWidth

	bar := progressFilled.Render(strings.Repeat("█", filledWidth)) +
		progressEmpty.Render(strings.Repeat("░", emptyWidth))

	return bar + progressLabel.Render(label)
}

func (m Model) renderCounters(w int) string {
	c := m.counts()
	var parts []string

	if c.ready > 0 {
		parts = append(parts, countReady.Render(fmt.Sprintf("⚡ %d ready", c.ready)))
	}
	if c.inflight > 0 {
		parts = append(parts, countInflight.Render(fmt.Sprintf("→ %d sent", c.inflight)))
	}
	if c.manual > 0 {
		parts = append(parts, countManual.Render(fmt.Sprintf("✋ %d manual", c.manual)))
	}
	if c.done > 0 {
		parts = append(parts, countDone.Render(fmt.Sprintf("✓ %d done", c.done)))
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "    ")
}

func renderSection(name string, width int) string {
	nameStr := sectionName.Render(name)
	nameWidth := lipgloss.Width(nameStr)
	ruleWidth := width - nameWidth - 4
	if ruleWidth < 4 {
		ruleWidth = 4
	}
	return "  " + nameStr + " " + sectionRule.Render(strings.Repeat("─", ruleWidth))
}

func renderCategoryHeader(cat taskCategory, w int) string {
	var pill string
	switch cat {
	case catReady:
		pill = catReadyPill.Render("⚡ Ready to dispatch")
	case catManual:
		pill = catManualPill.Render("✋ Needs you")
	case catInflight:
		pill = catInflightPill.Render("→ In-flight")
	case catDone:
		pill = catDonePill.Render("✓ Done") + "  " + dimStyle.Render("(v to collapse)")
	default:
		return ""
	}

	rule := sectionRule.Render(strings.Repeat("─", w))
	return pill + "\n" + rule
}

func (m Model) renderTask(displayPos, realIdx int, t taskfile.Task, w int) string {
	selected := displayPos == m.cursor

	cursor := "  "
	if selected {
		cursor = cursorStyle.Render("›") + " "
	}

	num := taskNum.Render(fmt.Sprintf("%-3s", fmt.Sprintf("%d.", realIdx+1)))
	check := checkOpen.Render("[ ]")
	if t.Status == taskfile.Done {
		check = checkDone.Render("[✓]")
	}

	badge := m.taskBadge(t)
	badgeWidth := lipgloss.Width(badge)
	prefix := cursor + num + " " + check + " "
	prefixWidth := lipgloss.Width(prefix)

	maxText := w - prefixWidth - badgeWidth - 2
	text := t.Text
	if maxText > 0 && len(text) > maxText {
		text = text[:maxText-1] + "…"
	}

	var styledText string
	switch {
	case selected:
		styledText = selectedText.Render(text)
	case t.Status == taskfile.Done:
		styledText = doneText.Render(text)
	default:
		styledText = text
	}

	left := prefix + styledText
	leftWidth := lipgloss.Width(left)
	padding := w - leftWidth - badgeWidth
	if padding < 1 {
		padding = 1
	}

	line := left + strings.Repeat(" ", padding) + badge

	if selected && t.Description != "" {
		for _, dl := range strings.Split(t.Description, "\n") {
			desc := truncate(dl, w-10)
			line += "\n" + "       " + dimStyle.Render("│ "+desc)
		}
	}

	return line
}

func (m Model) renderDoneTask(displayPos int, t taskfile.Task, w int) string {
	selected := displayPos == m.cursor

	cursor := "  "
	if selected {
		cursor = cursorStyle.Render("›") + " "
	}

	maxText := w - 8
	text := t.Text
	if maxText > 0 && len(text) > maxText {
		text = text[:maxText-1] + "…"
	}

	if selected {
		return cursor + checkDone.Render("✓") + " " + selectedText.Render(text)
	}
	return cursor + doneText.Render("✓ "+text)
}

func (m Model) taskBadge(t taskfile.Task) string {
	generic := t.Target == config.GenericRepoName
	isMCP := m.cfg.IsMCP(t.Target)
	switch {
	case t.Tag == taskfile.TagDispatched:
		return m.inflightBadge(t)
	case t.Tag == taskfile.TagAgent && generic:
		return badgeAgentGen.Render("generic ✦")
	case t.Tag == taskfile.TagAgent && isMCP:
		return badgeAgentMCP.Render(t.Target + " ◈")
	case t.Tag == taskfile.TagAgent && t.Target != "":
		return badgeAgent.Render(t.Target + " ⚡")
	case t.Tag == taskfile.TagAgent:
		return badgeAgent.Render("⚡")
	case t.Tag == taskfile.TagDraft && generic:
		return badgeDraftGen.Render("generic ✦")
	case t.Tag == taskfile.TagDraft && isMCP:
		return badgeDraftMCP.Render(t.Target + " ◈")
	case t.Tag == taskfile.TagDraft && t.Target != "":
		return badgeDraft.Render(t.Target + " ✎")
	case t.Tag == taskfile.TagDraft:
		return badgeDraft.Render("✎ draft")
	case t.Status == taskfile.Done:
		return badgeDone.Render("✓")
	default:
		return ""
	}
}

func (m Model) inflightBadge(t taskfile.Task) string {
	tr := m.trackerForTask(t)
	if tr == nil {
		return badgeDispatched.Render("→ sent")
	}

	elapsed := formatDuration(tr.Elapsed())

	switch tr.Status {
	case tracker.StatusDone:
		return badgeDone.Render(fmt.Sprintf("✓ done (%d turns)", tr.Turns))
	case tracker.StatusNeedsInput:
		return badgeNeedsInput.Render("⚠ needs input (r)")
	case tracker.StatusFailed:
		return errorStyle.Render("✗ failed")
	case tracker.StatusCrashed:
		return errorStyle.Render("✗ crashed")
	default:
		if tr.LastTool != "" {
			detail := tr.LastTool
			if tr.LastDetail != "" {
				detail = tr.LastDetail
			}
			return badgeDispatched.Render(fmt.Sprintf("%s  %s", detail, elapsed))
		}
		return badgeDispatched.Render(fmt.Sprintf("working…  %s", elapsed))
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}

func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func keyHint(key, desc string) string {
	return footerKey.Render(key) + footerDesc.Render(" "+desc)
}

func (m Model) renderFooter() string {
	var parts []string
	hasTasks := len(m.tasks) > 0

	parts = append(parts, keyHint("e", "edit"))
	if hasTasks {
		parts = append(parts, keyHint("f", "format"))
		parts = append(parts, keyHint("x", "done"))
		parts = append(parts, keyHint("a", "tag"))
		parts = append(parts, keyHint("A", "assign"))
		ac := m.agentCount()
		if ac > 0 {
			parts = append(parts, keyHint("d", fmt.Sprintf("dispatch(%d)", ac)))
		}
		parts = append(parts, keyHint("D", "delete"))
		c := m.counts()
		if c.inflight > 0 {
			parts = append(parts, keyHint("w", "watch"))
		}
		if t := m.currentTask(); t != nil && t.Tag == taskfile.TagDispatched {
			parts = append(parts, keyHint("r", "resume"))
		}
		if c.done > 0 {
			if m.doneExpanded {
				parts = append(parts, keyHint("v", "hide done"))
			} else {
				parts = append(parts, keyHint("v", fmt.Sprintf("done(%d)", c.done)))
			}
		}
	}
	if m.showHistory {
		parts = append(parts, keyHint("h", "hide sessions"))
	} else {
		parts = append(parts, keyHint("h", "sessions"))
	}
	parts = append(parts, keyHint("Tab", "settings"), keyHint("?", "help"))

	return strings.Join(parts, "  ")
}

func (m Model) loadingView() string {
	return fmt.Sprintf("\n  %s %s\n", m.spinner.View(), m.loadMsg)
}

func (m Model) dispatchConfirmView() string {
	if m.dispatchTarget == nil || m.dispatchIdx >= len(m.tasks) {
		return ""
	}

	task := m.tasks[m.dispatchIdx]
	w := m.contentWidth()
	generic := m.dispatchTarget.Name == config.GenericRepoName
	isMCP := m.cfg.IsMCP(m.dispatchTarget.Name)

	var b strings.Builder
	contentW := w - 4
	title := "Dispatch task"
	switch {
	case isMCP:
		title = "Dispatch task to MCP agent"
	case generic:
		title = "Dispatch task to generic workspace"
	}
	header := headerBox.Width(contentW).Render(
		lipgloss.NewStyle().Bold(true).Render(title),
	)
	b.WriteString(header)
	b.WriteString("\n\n")

	targetLabel := "Repo:"
	targetBadge := badgeAgent.Render(m.dispatchTarget.Name)
	switch {
	case isMCP:
		targetLabel = "MCP:"
		targetBadge = badgeAgentMCP.Render(m.dispatchTarget.Name + " ◈")
	case generic:
		targetLabel = "Target:"
		targetBadge = badgeAgentGen.Render(m.dispatchTarget.Name + " ✦")
	}

	b.WriteString("  " + lipgloss.NewStyle().Bold(true).Render("Task:") + " " + task.Text + "\n")
	b.WriteString("  " + lipgloss.NewStyle().Bold(true).Render(targetLabel) + " " + targetBadge + "\n")
	b.WriteString("  " + lipgloss.NewStyle().Bold(true).Render("Path:") + " " + dimStyle.Render(m.dispatchTarget.Path) + "\n")
	if m.dispatchTarget.Description != "" {
		b.WriteString("  " + lipgloss.NewStyle().Bold(true).Render("Desc:") + " " + dimStyle.Render(m.dispatchTarget.Description) + "\n")
	}
	if generic || isMCP {
		hint := "The workspace dir will be created if it doesn't exist."
		if isMCP {
			hint = "Runs in generic_workspace with an MCP-focused prompt. Requires " + m.dispatchTarget.Name + " in claude mcp list."
		}
		b.WriteString("\n  " + dimStyle.Render(hint) + "\n")
	}
	b.WriteString("\n")

	b.WriteString("  " + keyHint("Enter", "confirm") + "  " + keyHint("Esc", "cancel") + "\n")

	return b.String()
}

func (m Model) helpView() string {
	w := m.contentWidth()
	var b strings.Builder

	contentW := w - 4
	header := headerBox.Width(contentW).Render(
		lipgloss.NewStyle().Bold(true).Render("td — keybindings"),
	)
	b.WriteString(header)
	b.WriteString("\n")

	type binding struct{ key, desc string }
	sections := []struct {
		name     string
		bindings []binding
	}{
		{"Navigation", []binding{
			{"j / k", "move down / up"},
			{"g / G", "jump to first / last task"},
		}},
		{"Tasks", []binding{
			{"e", "open today's file in editor"},
			{"f", "format tasks with Claude"},
			{"x", "toggle done"},
			{"a", "cycle: none → agent → draft → none"},
			{"A", "assign target (repo, MCP, generic) with picker"},
			{"d", "dispatch @agent task (routes to repo, MCP, or generic)"},
			{"D / Ctrl+D", "delete task (press twice)"},
		}},
		{"Sessions (h)", []binding{
			{"h", "toggle sessions sidebar"},
			{"j / k", "select session"},
			{"w", "watch session log (read-only)"},
			{"Enter / r", "resume in iTerm (guard on running)"},
			{"L", "open session log in editor"},
			{"D", "remove session (press twice)"},
		}},
		{"General", []binding{
			{"Tab", "toggle settings view"},
			{"c", "open config in editor"},
			{"?", "toggle this help"},
			{"q", "quit"},
		}},
		{"Settings (Tab)", []binding{
			{"s", "scan directory for repos"},
			{"e", "edit config in editor"},
			{"D", "delete selected repo"},
		}},
	}

	for _, s := range sections {
		b.WriteString(helpSection.Render("  "+s.name) + "\n")
		for _, bind := range s.bindings {
			b.WriteString(fmt.Sprintf("  %s  %s\n",
				helpKey.Render(bind.key),
				helpDesc.Render(bind.desc)))
		}
	}

	b.WriteString("\n  " + keyHint("?", "close") + "  " + keyHint("Esc", "close") + "\n")
	return b.String()
}

func (m Model) scanInputView() string {
	w := m.contentWidth()
	var b strings.Builder

	contentW := w - 4
	header := headerBox.Width(contentW).Render(
		lipgloss.NewStyle().Bold(true).Render("Scan for repos"),
	)
	b.WriteString(header)
	b.WriteString("\n\n")

	b.WriteString("  Enter the directory to scan for git repositories:\n\n")

	inputLine := m.scanInput
	if inputLine == "" {
		inputLine = dimStyle.Render("~/Development/...")
	}
	b.WriteString(fmt.Sprintf("  > %s", inputLine))
	b.WriteString(cursorStyle.Render("█") + "\n")

	b.WriteString("\n  " + keyHint("Enter", "scan") + "  " + keyHint("Esc", "cancel") + "\n")
	return b.String()
}
