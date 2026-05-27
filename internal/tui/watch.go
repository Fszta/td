package tui

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"td/internal/tracker"
)

var (
	watchInitStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("62")).Bold(true)
	watchToolStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	watchDoneStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("77")).Bold(true)
	watchFailStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	watchIndentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	watchStatusBar   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	watchStatusKey   = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	watchTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252"))
	watchToolName    = lipgloss.NewStyle().Bold(true)
)

type watchTickMsg struct{}

// watchEvent is a minimal representation of a Claude stream-json event for watch rendering.
type watchEvent struct {
	Type         string          `json:"type"`
	Subtype      string          `json:"subtype,omitempty"`
	SessionID    string          `json:"session_id,omitempty"`
	Model        string          `json:"model,omitempty"`
	Message      *watchMessage   `json:"message,omitempty"`
	Content      json.RawMessage `json:"content,omitempty"`
	TotalCostUSD float64         `json:"total_cost_usd,omitempty"`
	DurationMs   int64           `json:"duration_ms,omitempty"`
	NumTurns     int             `json:"num_turns,omitempty"`
	Error        string          `json:"error,omitempty"`
}

type watchMessage struct {
	Content []watchContentBlock `json:"content"`
}

type watchContentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

// WatchModel streams the .jsonl log of a running or completed session.
type WatchModel struct {
	task       *tracker.Task
	logPath    string
	lines      []string
	byteOffset int64
	viewport   viewport.Model
	width      int
	height     int
	atBottom   bool
	logMissing bool
}

// viewportDims returns the usable viewport width and height for the watch overlay.
func viewportDims(width, height int) (vpW, vpH int) {
	return max(width-4, 20), max(height-6, 3)
}

func newWatchModel(task *tracker.Task, logPath string, width, height int) WatchModel {
	vpW, vpH := viewportDims(width, height)
	vp := viewport.New(vpW, vpH)
	w := WatchModel{
		task:     task,
		logPath:  logPath,
		viewport: vp,
		width:    width,
		height:   height,
		atBottom: true,
	}
	lines, offset, missing := readWatchLines(logPath, 0)
	w.logMissing = missing
	if len(lines) > 0 {
		w.lines = lines
		w.byteOffset = offset
		w.viewport.SetContent(strings.Join(lines, "\n"))
		w.viewport.GotoBottom()
	}
	return w
}

func (w WatchModel) init() tea.Cmd {
	return func() tea.Msg { return watchTickMsg{} }
}

func (w WatchModel) onTick() (WatchModel, tea.Cmd) {
	newLines, newOffset, missing := readWatchLines(w.logPath, w.byteOffset)
	w.logMissing = missing
	if len(newLines) > 0 {
		wasAtBottom := w.atBottom
		w.lines = append(w.lines, newLines...)
		w.viewport.SetContent(strings.Join(w.lines, "\n"))
		if wasAtBottom {
			w.viewport.GotoBottom()
			w.atBottom = true
		}
	}
	w.byteOffset = newOffset

	if w.task != nil && w.task.Status == tracker.StatusRunning {
		return w, tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
			return watchTickMsg{}
		})
	}
	return w, nil
}

func (w WatchModel) handleKey(msg tea.KeyMsg) (WatchModel, tea.Cmd) {
	var cmd tea.Cmd
	w.viewport, cmd = w.viewport.Update(msg)
	w.atBottom = w.viewport.AtBottom()
	return w, cmd
}

func (w WatchModel) resize(width, height int) WatchModel {
	w.width = width
	w.height = height
	vpW, vpH := viewportDims(width, height)
	w.viewport.Width = vpW
	w.viewport.Height = vpH
	w.viewport.SetContent(strings.Join(w.lines, "\n"))
	if w.atBottom {
		w.viewport.GotoBottom()
	}
	return w
}

func (w WatchModel) view() string {
	if w.logMissing {
		var b strings.Builder
		b.WriteString("\n  ")
		b.WriteString(errorStyle.Render("Log file not found"))
		b.WriteString("\n  ")
		b.WriteString(dimStyle.Render(w.logPath))
		b.WriteString("\n\n  ")
		b.WriteString(keyHint("q", "back"))
		b.WriteString("\n")
		return frameStyle.Width(w.width - 4).Render(b.String())
	}

	var b strings.Builder

	// Header
	title := "Watch"
	if w.task != nil {
		title = "Watch · " + sessionTitle(w.task.TaskText, 50)
	}
	b.WriteString(watchTitleStyle.Render(title))
	b.WriteString("\n")
	b.WriteString(historyBorderStyle.Render(strings.Repeat("─", w.viewport.Width)))
	b.WriteString("\n")

	// Log viewport
	b.WriteString(w.viewport.View())
	b.WriteString("\n")

	// Status bar
	b.WriteString(historyBorderStyle.Render(strings.Repeat("─", w.viewport.Width)))
	b.WriteString("\n")
	b.WriteString(w.renderStatusBar())
	b.WriteString("\n")
	b.WriteString(keyHint("q", "back") + "  " + keyHint("↑↓", "scroll") + "  " + keyHint("G", "bottom"))

	return frameStyle.Width(w.width - 4).Render(b.String())
}

func (w WatchModel) renderStatusBar() string {
	if w.task == nil {
		return ""
	}
	t := w.task
	statusStyled := watchStatusKey.Render("[" + sessionStatusShort(t.Status) + "]")
	title := sessionTitle(t.TaskText, 28)
	repo := t.RepoName

	parts := []string{statusStyled, watchStatusBar.Render(title), watchStatusBar.Render("repo: " + repo)}
	if t.Turns > 0 {
		parts = append(parts, watchStatusBar.Render(fmt.Sprintf("%d turns", t.Turns)))
	}
	if t.CostUSD > 0 {
		parts = append(parts, watchStatusBar.Render(fmt.Sprintf("$%.3f", t.CostUSD)))
	}
	parts = append(parts, watchStatusBar.Render(formatDuration(t.Elapsed())))

	return strings.Join(parts, watchStatusBar.Render(" · "))
}

// readWatchLines reads new JSONL lines from logPath starting at offset, returning rendered display lines.
func readWatchLines(logPath string, offset int64) (lines []string, newOffset int64, missing bool) {
	f, err := os.Open(logPath)
	if err != nil {
		return nil, offset, true
	}
	defer f.Close()

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, offset, false
	}

	data, err := io.ReadAll(f)
	if err != nil || len(data) == 0 {
		return nil, offset, false
	}

	newOffset = offset + int64(len(data))

	for _, raw := range strings.Split(string(data), "\n") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		lines = append(lines, parseWatchLine(raw)...)
	}

	return lines, newOffset, false
}

// parseWatchLine maps a single JSONL event to zero or more display lines.
func parseWatchLine(raw string) []string {
	var ev watchEvent
	if json.Unmarshal([]byte(raw), &ev) != nil {
		return nil
	}

	switch ev.Type {
	case "system":
		text := watchInitStyle.Render("◆") + " Session started"
		if ev.Model != "" {
			text += " — model: " + dimStyle.Render(ev.Model)
		}
		if ev.SessionID != "" {
			short := ev.SessionID
			if len(short) > 8 {
				short = short[:8]
			}
			text += " · " + dimStyle.Render("session: "+short)
		}
		return []string{text, ""}

	case "assistant":
		if ev.Message == nil {
			return nil
		}
		var out []string
		for _, block := range ev.Message.Content {
			switch block.Type {
			case "text":
				if block.Text != "" {
					for _, line := range strings.Split(strings.TrimRight(block.Text, "\n"), "\n") {
						if strings.TrimSpace(line) != "" {
							out = append(out, "  "+line)
						}
					}
					out = append(out, "")
				}
			case "tool_use":
				summary := watchSummarizeInput(block.Name, block.Input)
				line := watchToolStyle.Render("▶") + " " + watchToolName.Render(block.Name)
				if summary != "" {
					line += "  " + dimStyle.Render(summary)
				}
				out = append(out, line)
			}
		}
		return out

	case "tool":
		result := watchParseContent(ev.Content)
		if result != "" {
			short := truncate(strings.ReplaceAll(result, "\n", " "), 100)
			return []string{"  " + watchIndentStyle.Render("└") + " " + dimStyle.Render(short)}
		}
		return nil

	case "result":
		// Claude CLI emits subtype:"success" on clean completion; the
		// (subtype=="" && num_turns>0) branch handles older schema versions
		// that omit subtype entirely.
		if ev.Subtype == "success" || (ev.Subtype == "" && ev.NumTurns > 0) {
			cost := fmt.Sprintf("$%.3f", ev.TotalCostUSD)
			dur := formatDuration(time.Duration(ev.DurationMs) * time.Millisecond)
			return []string{"", watchDoneStyle.Render("✓") + fmt.Sprintf(" Done — %d turns · %s · %s", ev.NumTurns, cost, dur)}
		}
		errMsg := ev.Error
		if errMsg == "" {
			errMsg = ev.Subtype
		}
		if errMsg == "" {
			errMsg = "unknown error"
		}
		return []string{"", watchFailStyle.Render("✗") + " Failed — " + dimStyle.Render(errMsg)}
	}

	return nil
}

func watchSummarizeInput(tool string, input json.RawMessage) string {
	var m map[string]any
	if json.Unmarshal(input, &m) != nil {
		return ""
	}
	switch tool {
	case "Edit", "Write", "Read":
		if p, ok := m["file_path"].(string); ok {
			return filepath.Base(p)
		}
		if p, ok := m["path"].(string); ok {
			return filepath.Base(p)
		}
	case "Bash":
		if cmd, ok := m["command"].(string); ok {
			return truncate(cmd, 60)
		}
	case "Grep", "Glob":
		if p, ok := m["pattern"].(string); ok {
			return truncate(p, 60)
		}
	case "WebSearch":
		if q, ok := m["query"].(string); ok {
			return truncate(q, 60)
		}
	case "WebFetch":
		if u, ok := m["url"].(string); ok {
			return truncate(u, 60)
		}
	}
	return ""
}

func watchParseContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text,omitempty"`
	}
	if json.Unmarshal(raw, &blocks) == nil {
		for _, b := range blocks {
			if b.Text != "" {
				return b.Text
			}
		}
	}
	return ""
}
