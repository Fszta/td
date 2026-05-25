package scan

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"td/internal/config"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("62")).Padding(0, 1)
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	successStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("77")).Italic(true)
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	checkStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("77"))
	uncheckStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
)

type scanState int

const (
	stateScanning scanState = iota
	stateDescribing
	stateSelecting
	stateDone
)

type reposFoundMsg struct {
	paths []string
	err   error
}

type reposDescribedMsg struct {
	suggestions []Suggestion
	err         error
}

type ScanModel struct {
	dir         string
	apiKey      string
	apiModel    string
	existing    map[string]bool
	state       scanState
	suggestions []Suggestion
	cursor      int
	spinner     spinner.Model
	err         error
	savedCount  int
}

func NewScanModel(dir, apiKey, apiModel string, existing map[string]bool) ScanModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = successStyle

	return ScanModel{
		dir:      dir,
		apiKey:   apiKey,
		apiModel: apiModel,
		existing: existing,
		state:    stateScanning,
		spinner:  s,
	}
}

func (m ScanModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.findRepos())
}

func (m ScanModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.state == stateSelecting {
			return m.handleSelectKey(msg)
		}
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}
		if m.state == stateDone {
			return m, tea.Quit
		}

	case reposFoundMsg:
		if msg.err != nil {
			m.err = msg.err
			m.state = stateDone
			return m, nil
		}
		if len(msg.paths) == 0 {
			m.err = fmt.Errorf("no git repositories found in %s", m.dir)
			m.state = stateDone
			return m, nil
		}
		m.state = stateDescribing
		return m, tea.Batch(m.spinner.Tick, m.describeRepos(msg.paths))

	case reposDescribedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.state = stateDone
			return m, nil
		}
		m.suggestions = FilterNew(msg.suggestions, m.existing)
		if len(m.suggestions) == 0 {
			m.err = fmt.Errorf("all discovered repos are already in your config")
			m.state = stateDone
			return m, nil
		}
		m.state = stateSelecting
		return m, nil

	case spinner.TickMsg:
		if m.state == stateScanning || m.state == stateDescribing {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m ScanModel) handleSelectKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.cursor < len(m.suggestions)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case " ", "x":
		m.suggestions[m.cursor].Selected = !m.suggestions[m.cursor].Selected
	case "enter":
		if err := SaveSelected(m.suggestions); err != nil {
			m.err = err
			m.state = stateDone
			return m, nil
		}
		count := 0
		for _, s := range m.suggestions {
			if s.Selected {
				count++
			}
		}
		m.savedCount = count
		m.state = stateDone
		return m, nil
	case "q", "esc", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m ScanModel) View() string {
	switch m.state {
	case stateScanning:
		return fmt.Sprintf("\n  %s Scanning %s for repositories...\n", m.spinner.View(), m.dir)
	case stateDescribing:
		return fmt.Sprintf("\n  %s Analyzing repositories with Claude...\n", m.spinner.View())
	case stateSelecting:
		return m.selectView()
	case stateDone:
		return m.doneView()
	}
	return ""
}

func (m ScanModel) selectView() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(titleStyle.Render(fmt.Sprintf(" td scan — found %d new repos ", len(m.suggestions))) + "\n\n")

	for i, s := range m.suggestions {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}

		check := checkStyle.Render("[x]")
		if !s.Selected {
			check = uncheckStyle.Render("[ ]")
		}

		name := fmt.Sprintf("%-24s", s.Name)
		line := fmt.Sprintf("%s%s %s %s", cursor, check, name, s.Description)
		if i == m.cursor {
			b.WriteString(selectedStyle.Render(line) + "\n")
		} else {
			b.WriteString("  " + line + "\n")
		}
	}

	b.WriteString("\n" + helpStyle.Render("  Space/x: toggle   Enter: save to config   q: cancel") + "\n")
	return b.String()
}

func (m ScanModel) doneView() string {
	if m.err != nil {
		return "\n" + errorStyle.Render(fmt.Sprintf("  Error: %s", m.err)) + "\n\n  press any key to exit\n"
	}
	if m.savedCount == 0 {
		return "\n" + dimStyle.Render("  No repos saved.") + "\n\n  press any key to exit\n"
	}
	return "\n" + successStyle.Render(fmt.Sprintf("  Saved %d repos to %s", m.savedCount, config.ConfigPath())) + "\n\n  press any key to exit\n"
}

func (m ScanModel) findRepos() tea.Cmd {
	dir := m.dir
	return func() tea.Msg {
		paths, err := FindRepos(dir)
		return reposFoundMsg{paths: paths, err: err}
	}
}

func (m ScanModel) describeRepos(paths []string) tea.Cmd {
	apiKey := m.apiKey
	apiModel := m.apiModel
	return func() tea.Msg {
		suggestions, err := DescribeRepos(paths, apiKey, apiModel)
		return reposDescribedMsg{suggestions: suggestions, err: err}
	}
}
