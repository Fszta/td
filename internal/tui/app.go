package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"td/internal/ai"
	"td/internal/config"
	"td/internal/taskfile"
	"td/internal/tracker"
)

type editorFinishedMsg struct{ err error }

type fileReloadedMsg struct {
	tasks []taskfile.Task
	err   error
}

type formatDoneMsg struct {
	content string
	err     error
}

type routeDoneMsg struct {
	result  *ai.RouteResult
	taskIdx int
	err     error
}

type dispatchDoneMsg struct {
	repoName string
	taskIdx  int
	err      error
}

type configReloadedMsg struct {
	cfg *config.Config
	err error
}

type trackerTickMsg struct{}
type trackerPollMsg struct {
	tasks map[string]*tracker.Task
}

type dstate int

const (
	dstateNone dstate = iota
	dstateConfirm
	dstatePicker
)

type viewMode int

const (
	viewTasks viewMode = iota
	viewSettings
)

type taskCategory int

const (
	catReady taskCategory = iota
	catManual
	catInflight
	catDone // omitted from displayOrder until expanded
)

type Model struct {
	cfg          *config.Config
	filePath     string
	tasks        []taskfile.Task
	displayOrder []int // indices into tasks, grouped by category
	cursor       int
	width        int
	height       int

	spinner  spinner.Model
	loading  bool
	loadMsg  string
	status   string
	err      error
	autoEdit bool

	dispatch       dstate
	dispatchTarget *config.Repo
	dispatchIdx    int

	pickerInput       string
	pickerCursor      int
	pickerForDispatch bool

	viewMode        viewMode
	showHelp        bool
	settingsCursor  int
	confirmDelete   bool
	editingConfig   bool
	confirmTaskDel  bool

	scanMode  bool
	scanInput string

	doneExpanded bool

	trackerStore      *tracker.Store
	trackedTasks      map[string]*tracker.Task
	showHistory       bool
	historyCursor     int
	confirmHistoryDel bool
	historySessions   []*tracker.Task
	pendingResumeID   string

	watchMode  bool
	watchModel WatchModel
}

func NewModel(cfg *config.Config, filePath string, tasks []taskfile.Task, autoEdit bool) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = statusStyle

	store, _ := tracker.NewStore()

	m := Model{
		cfg:          cfg,
		filePath:     filePath,
		tasks:        tasks,
		spinner:      s,
		autoEdit:     autoEdit,
		width:        80,
		trackerStore: store,
		trackedTasks: make(map[string]*tracker.Task),
	}
	m.rebuildOrder()
	m.refreshHistorySessions()
	return m
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.pollTrackers()}
	if m.autoEdit {
		cmds = append(cmds, m.openEditor())
	}
	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.watchMode {
			m.watchModel = m.watchModel.resize(msg.Width, msg.Height)
		}
		if m.showHistory && !m.canShowHistorySidebar() {
			m.showHistory = false
		}
		return m, nil

	case watchTickMsg:
		if m.watchMode {
			var cmd tea.Cmd
			m.watchModel, cmd = m.watchModel.onTick()
			return m, cmd
		}
		return m, nil

	case tea.KeyMsg:
		if m.loading {
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			return m, nil
		}
		if m.showHelp {
			if msg.String() == "?" || msg.String() == "esc" || msg.String() == "q" {
				m.showHelp = false
			}
			return m, nil
		}
		if m.watchMode {
			if msg.String() == "q" || msg.String() == "esc" {
				m.watchMode = false
				return m, nil
			}
			var cmd tea.Cmd
			m.watchModel, cmd = m.watchModel.handleKey(msg)
			return m, cmd
		}
		if m.dispatch == dstateConfirm {
			return m.handleDispatchKey(msg)
		}
		if m.dispatch == dstatePicker {
			return m.handlePickerKey(msg)
		}
		if m.scanMode {
			return m.handleScanInputKey(msg)
		}
		if m.viewMode == viewSettings {
			return m.handleSettingsKey(msg)
		}
		m.err = nil
		if msg.String() != "D" && msg.String() != "ctrl+d" {
			m.confirmTaskDel = false
		}
		return m.handleKey(msg)

	case editorFinishedMsg:
		m.editingConfig = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		return m, tea.Batch(m.reloadFile(), m.reloadConfig())

	case fileReloadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.tasks = msg.tasks
		m.rebuildOrder()
		if m.cursor >= len(m.displayOrder) {
			m.cursor = max(0, len(m.displayOrder)-1)
		}
		return m, nil

	case configReloadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.cfg = msg.cfg
		if m.settingsCursor >= len(m.cfg.Repos) {
			m.settingsCursor = max(0, len(m.cfg.Repos)-1)
		}
		m.status = "Config reloaded"
		return m, nil

	case formatDoneMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		if err := taskfile.WriteContent(m.filePath, msg.content); err != nil {
			m.err = err
			return m, nil
		}
		m.status = "Formatted"
		m.cursor = 0
		return m, m.reloadFile()

	case routeDoneMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		r := msg.result
		if !r.Actionable {
			hint := "Task needs more context"
			if r.Missing != "" {
				hint += " — " + r.Missing
			}
			m.err = fmt.Errorf("%s", hint)
			return m, nil
		}
		var repo *config.Repo
		switch {
		case r.MCP != "":
			repo = m.findRepo(r.MCP)
		case r.Generic:
			repo = m.genericRepo()
		default:
			repo = m.findRepo(r.Repo)
		}
		if repo == nil {
			m.status = "No repo match — pick one"
			m.dispatchIdx = msg.taskIdx
			m.dispatch = dstatePicker
			m.pickerInput = ""
			m.pickerCursor = 0
			m.pickerForDispatch = true
			return m, nil
		}
		m.dispatchTarget = repo
		m.dispatchIdx = msg.taskIdx
		m.dispatch = dstateConfirm
		return m, nil

	case dispatchDoneMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			m.dispatchTarget = nil
			return m, nil
		}
		t := &m.tasks[msg.taskIdx]
		t.Tag = taskfile.TagDispatched
		t.Target = ""
		if err := taskfile.UpdateTask(m.filePath, *t); err != nil {
			m.err = err
		} else {
			m.status = fmt.Sprintf("Dispatched to %s", msg.repoName)
		}
		m.dispatchTarget = nil
		m.rebuildOrder()
		return m, m.scheduleTrackerTick()

	case trackerTickMsg:
		return m, m.pollTrackers()

	case trackerPollMsg:
		m.trackedTasks = msg.tasks
		m.refreshHistorySessions()
		if m.watchMode && m.watchModel.task != nil {
			if t, ok := msg.tasks[m.watchModel.task.ID]; ok {
				m.watchModel.task = t
			}
		}
		return m, m.scheduleTrackerTick()

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.showHistory {
		return m.handleHistoryKey(msg)
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "?":
		m.showHelp = true
		return m, nil

	case "h":
		if m.canShowHistorySidebar() {
			m.showHistory = true
			m.historyCursor = 0
			m.confirmHistoryDel = false
			m.refreshHistorySessions()
		} else {
			m.status = fmt.Sprintf("Need at least %d columns for sessions panel", historyMinTermWidth)
		}
		return m, nil

	case "tab":
		m.viewMode = viewSettings
		m.confirmDelete = false
		return m, nil

	case "j", "down":
		if m.cursor < len(m.displayOrder)-1 {
			m.cursor++
		}
		m.pendingResumeID = ""

	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
		m.pendingResumeID = ""

	case "G":
		if len(m.displayOrder) > 0 {
			m.cursor = len(m.displayOrder) - 1
		}
		m.pendingResumeID = ""

	case "g":
		m.cursor = 0
		m.pendingResumeID = ""

	case "e":
		return m, m.openEditor()

	case "c":
		m.editingConfig = true
		return m, m.openConfig()

	case "x":
		t := m.currentTask()
		if t == nil {
			return m, nil
		}
		if t.Status == taskfile.Open {
			t.Status = taskfile.Done
		} else {
			t.Status = taskfile.Open
		}
		if err := taskfile.UpdateTask(m.filePath, *t); err != nil {
			m.err = err
		}
		m.rebuildOrder()

	case "v":
		m.doneExpanded = !m.doneExpanded
		m.rebuildOrder()
		if m.cursor >= len(m.displayOrder) && len(m.displayOrder) > 0 {
			m.cursor = len(m.displayOrder) - 1
		}
		return m, nil

	case "w":
		m.pendingResumeID = ""
		if m.trackerStore == nil {
			return m, nil
		}
		var tr *tracker.Task
		if t := m.currentTask(); t != nil && t.Tag == taskfile.TagDispatched {
			tr = m.trackerForTask(*t)
		}
		if tr == nil {
			// No dispatched task selected: fall back to the most recently touched
			// session so that pressing w from the task list always opens something
			// useful when only one session is active.
			for _, s := range m.trackerStore.ListRecent(1) {
				tr = s
				break
			}
		}
		if tr == nil {
			m.err = fmt.Errorf("no session to watch")
			return m, nil
		}
		logPath := m.trackerStore.LogPath(tr.ID)
		m.watchMode = true
		m.watchModel = newWatchModel(tr, logPath, m.width, m.height)
		return m, m.watchModel.init()

	case "r":
		t := m.currentTask()
		if t == nil || t.Tag != taskfile.TagDispatched {
			return m, nil
		}
		tr := m.trackerForTask(*t)
		if tr == nil || tr.SessionID == "" {
			m.err = fmt.Errorf("no session to resume")
			return m, nil
		}
		m, ok := m.confirmResume(tr)
		if !ok {
			return m, nil
		}
		return m.resumeTrackerSession(tr)

	case "D", "ctrl+d":
		t := m.currentTask()
		if t == nil {
			return m, nil
		}
		if m.confirmTaskDel {
			if err := taskfile.DeleteTask(m.filePath, *t); err != nil {
				m.err = err
			} else {
				m.status = "Deleted"
			}
			m.confirmTaskDel = false
			return m, m.reloadFile()
		}
		m.confirmTaskDel = true
		return m, nil

	case "a":
		t := m.currentTask()
		if t == nil {
			return m, nil
		}
		switch t.Tag {
		case taskfile.TagNone:
			t.Tag = taskfile.TagAgent
		case taskfile.TagAgent:
			t.Tag = taskfile.TagDraft
		case taskfile.TagDraft:
			t.Tag = taskfile.TagNone
			t.Target = ""
		default:
			return m, nil
		}
		if err := taskfile.UpdateTask(m.filePath, *t); err != nil {
			m.err = err
		}
		m.rebuildOrder()

	case "A":
		if len(m.tasks) == 0 {
			return m, nil
		}
		m.dispatch = dstatePicker
		m.pickerInput = ""
		m.pickerCursor = 0
		m.pickerForDispatch = false
		return m, nil

	case "f":
		if m.cfg.AnthropicAPIKey == "" {
			m.err = fmt.Errorf("set ANTHROPIC_API_KEY or add to config")
			return m, nil
		}
		m.loading = true
		m.loadMsg = "Formatting with Claude..."
		m.status = ""
		return m, tea.Batch(m.spinner.Tick, m.formatTasks())

	case "d":
		t := m.currentTask()
		if t == nil {
			return m, nil
		}
		if t.Tag != taskfile.TagAgent && t.Tag != taskfile.TagDraft {
			m.err = fmt.Errorf("select an @agent or @draft task to dispatch")
			return m, nil
		}
		realIdx := m.taskIdx()
		if t.Target != "" {
			repo := m.findRepo(t.Target)
			if repo == nil {
				m.err = fmt.Errorf("target %q not found in config (repos, mcp, or generic)", t.Target)
				return m, nil
			}
			m.dispatchTarget = repo
			m.dispatchIdx = realIdx
			m.dispatch = dstateConfirm
			return m, nil
		}
		if m.cfg.AnthropicAPIKey == "" {
			m.err = fmt.Errorf("set ANTHROPIC_API_KEY for routing")
			return m, nil
		}
		m.loading = true
		m.loadMsg = "Routing task..."
		m.status = ""
		return m, tea.Batch(m.spinner.Tick, m.routeTask(realIdx))
	}

	return m, nil
}

func (m Model) handleDispatchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		repoName := m.dispatchTarget.Name
		m.dispatch = dstateNone
		m.loading = true
		m.loadMsg = fmt.Sprintf("Launching in %s...", repoName)
		return m, tea.Batch(m.spinner.Tick, m.launchDispatch())
	case "esc":
		m.dispatch = dstateNone
		m.dispatchTarget = nil
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleScanInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		dir := strings.TrimSpace(m.scanInput)
		if dir == "" {
			return m, nil
		}
		m.scanMode = false
		return m, m.launchScan(dir)
	case "esc":
		m.scanMode = false
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "backspace":
		if len(m.scanInput) > 0 {
			m.scanInput = m.scanInput[:len(m.scanInput)-1]
		}
	default:
		r := msg.String()
		if len(r) == 1 {
			m.scanInput += r
		}
	}
	return m, nil
}

func (m Model) openEditor() tea.Cmd {
	c := exec.Command(m.cfg.Editor, m.filePath)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{err: err}
	})
}

func (m Model) openConfig() tea.Cmd {
	configPath, _ := config.EnsureConfigFile()
	if configPath == "" {
		return nil
	}
	c := exec.Command(m.cfg.Editor, configPath)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{err: err}
	})
}

func (m Model) reloadFile() tea.Cmd {
	path := m.filePath
	return func() tea.Msg {
		tasks, err := taskfile.Parse(path)
		return fileReloadedMsg{tasks: tasks, err: err}
	}
}

func (m Model) reloadConfig() tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load()
		return configReloadedMsg{cfg: cfg, err: err}
	}
}

func (m Model) launchScan(dir string) tea.Cmd {
	if strings.HasPrefix(dir, "~/") {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, dir[2:])
	} else if dir == "~" {
		home, _ := os.UserHomeDir()
		dir = home
	}
	exe, err := os.Executable()
	if err != nil {
		exe = os.Args[0]
	}
	c := exec.Command(exe, "scan", dir)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{err: err}
	})
}

func (m Model) formatTasks() tea.Cmd {
	path := m.filePath
	apiKey := m.cfg.AnthropicAPIKey
	apiModel := m.cfg.AnthropicModel
	repos := m.cfg.Repos
	mcps := m.cfg.MCPs
	return func() tea.Msg {
		content, err := taskfile.ReadContent(path)
		if err != nil {
			return formatDoneMsg{err: err}
		}
		client := ai.NewClient(apiKey, apiModel)
		result, err := client.Format(content, repos, mcps)
		return formatDoneMsg{content: result, err: err}
	}
}

func (m Model) routeTask(idx int) tea.Cmd {
	task := m.tasks[idx]
	apiKey := m.cfg.AnthropicAPIKey
	apiModel := m.cfg.AnthropicModel
	repos := m.cfg.Repos
	mcps := m.cfg.MCPs
	return func() tea.Msg {
		client := ai.NewClient(apiKey, apiModel)
		result, err := client.Route(task.FullText(), repos, mcps)
		return routeDoneMsg{result: result, taskIdx: idx, err: err}
	}
}

func (m Model) launchDispatch() tea.Cmd {
	target := *m.dispatchTarget
	task := m.tasks[m.dispatchIdx]
	idx := m.dispatchIdx
	store := m.trackerStore
	mcps := m.cfg.MCPs
	return func() tea.Msg {
		if store == nil {
			return dispatchDoneMsg{repoName: target.Name, taskIdx: idx, err: fmt.Errorf("tracker store not initialized")}
		}
		_, err := launchHeadless(store, target.Path, target.Name, task.FullText(), mcps)
		return dispatchDoneMsg{repoName: target.Name, taskIdx: idx, err: err}
	}
}

func (m Model) findRepo(name string) *config.Repo {
	if strings.EqualFold(name, config.GenericRepoName) {
		return m.genericRepo()
	}
	if mcp := m.cfg.FindMCP(name); mcp != nil {
		return m.mcpRepo(mcp)
	}
	for i := range m.cfg.Repos {
		if strings.EqualFold(m.cfg.Repos[i].Name, name) {
			return &m.cfg.Repos[i]
		}
	}
	return nil
}

// genericRepo returns a synthetic Repo pointing at the configured generic
// workspace. The result is not part of m.cfg.Repos — it's a stand-in so the
// dispatch flow can treat the generic destination uniformly.
func (m Model) genericRepo() *config.Repo {
	return &config.Repo{
		Name:        config.GenericRepoName,
		Path:        m.cfg.GenericWorkspace,
		Description: "Generic workspace for tasks without a repo home",
	}
}

// mcpRepo returns a synthetic Repo for MCP dispatch (runs in generic_workspace).
func (m Model) mcpRepo(mcp *config.MCP) *config.Repo {
	desc := mcp.Description
	if desc != "" {
		desc = "MCP — " + desc
	} else {
		desc = "MCP agent target"
	}
	return &config.Repo{
		Name:        mcp.Name,
		Path:        m.cfg.GenericWorkspace,
		Description: desc,
	}
}

func (m Model) contentWidth() int {
	w := m.width
	if w < 60 {
		w = 80
	}
	return w
}

func (m Model) innerWidth() int {
	w := m.contentWidth() - 4
	if w < 40 {
		w = 40
	}
	return w
}

func (m Model) taskCounts() (int, int) {
	done := 0
	for _, t := range m.tasks {
		if t.Status == taskfile.Done {
			done++
		}
	}
	return done, len(m.tasks)
}

type statusCounts struct {
	ready, manual, inflight, done int
}

func (m Model) counts() statusCounts {
	var c statusCounts
	for _, t := range m.tasks {
		switch taskCat(t) {
		case catReady:
			c.ready++
		case catManual:
			c.manual++
		case catInflight:
			c.inflight++
		case catDone:
			c.done++
		}
	}
	return c
}

func (m Model) agentCount() int {
	n := 0
	for _, t := range m.tasks {
		if (t.Tag == taskfile.TagAgent || t.Tag == taskfile.TagDraft) && t.Status == taskfile.Open {
			n++
		}
	}
	return n
}

func taskCat(t taskfile.Task) taskCategory {
	if t.Status == taskfile.Done {
		return catDone
	}
	if t.Tag == taskfile.TagDispatched {
		return catInflight
	}
	if t.Tag == taskfile.TagAgent || t.Tag == taskfile.TagDraft {
		return catReady
	}
	return catManual
}

func (m *Model) rebuildOrder() {
	var ready, manual, inflight, done []int
	for i, t := range m.tasks {
		switch taskCat(t) {
		case catReady:
			ready = append(ready, i)
		case catManual:
			manual = append(manual, i)
		case catInflight:
			inflight = append(inflight, i)
		case catDone:
			done = append(done, i)
		}
	}
	m.displayOrder = nil
	m.displayOrder = append(m.displayOrder, ready...)
	m.displayOrder = append(m.displayOrder, manual...)
	m.displayOrder = append(m.displayOrder, inflight...)
	if m.doneExpanded {
		m.displayOrder = append(m.displayOrder, done...)
	}
}

// taskIdx returns the real index into m.tasks for the current cursor position.
func (m Model) taskIdx() int {
	if m.cursor < 0 || m.cursor >= len(m.displayOrder) {
		return -1
	}
	return m.displayOrder[m.cursor]
}

func (m Model) currentTask() *taskfile.Task {
	idx := m.taskIdx()
	if idx < 0 {
		return nil
	}
	return &m.tasks[idx]
}

func (m Model) scheduleTrackerTick() tea.Cmd {
	return tea.Tick(5*time.Second, func(time.Time) tea.Msg {
		return trackerTickMsg{}
	})
}

func (m Model) pollTrackers() tea.Cmd {
	store := m.trackerStore
	return func() tea.Msg {
		if store == nil {
			return trackerPollMsg{tasks: make(map[string]*tracker.Task)}
		}
		all := store.AllTasks()
		for _, t := range all {
			if t.Status == tracker.StatusRunning {
				store.Poll(t)
				_ = store.Update(t)
			}
		}
		return trackerPollMsg{tasks: all}
	}
}

func (m Model) trackerForTask(t taskfile.Task) *tracker.Task {
	if t.Tag != taskfile.TagDispatched {
		return nil
	}
	for _, tr := range m.trackedTasks {
		if tr.TaskText == t.FullText() || tr.TaskText == t.Text {
			return tr
		}
	}
	return nil
}

// confirmResume gates resume on running sessions: first press warns, second confirms.
func (m Model) confirmResume(tr *tracker.Task) (Model, bool) {
	if tr.Status != tracker.StatusRunning {
		m.pendingResumeID = ""
		return m, true
	}
	if m.pendingResumeID != tr.ID {
		m.pendingResumeID = tr.ID
		m.status = "Session running — press r again to confirm, or w to watch"
		m.err = nil
		return m, false
	}
	m.pendingResumeID = ""
	return m, true
}

func (m Model) resumeTrackerSession(tr *tracker.Task) (tea.Model, tea.Cmd) {
	if tr == nil {
		return m, nil
	}
	if tr.SessionID == "" {
		m.err = fmt.Errorf("no session to resume")
		return m, nil
	}
	if tracker.IsProcessAlive(tr.PID) {
		proc, _ := os.FindProcess(tr.PID)
		if proc != nil {
			_ = proc.Kill()
		}
	}
	if err := resumeInITerm(tr); err != nil {
		m.err = fmt.Errorf("resume: %w", err)
	} else {
		m.status = fmt.Sprintf("Resumed in iTerm (%s)", tr.RepoName)
		m.err = nil
	}
	return m, nil
}
