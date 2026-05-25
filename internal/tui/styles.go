package tui

import "github.com/charmbracelet/lipgloss"

var frameStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("62")).
	Padding(0, 1)

var headerBox = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("62")).
	Padding(0, 1)

var separatorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))

var (
	progressFilled = lipgloss.NewStyle().Foreground(lipgloss.Color("62"))
	progressEmpty  = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	progressLabel  = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
)

var (
	countReady    = lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Bold(true)
	countManual   = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Bold(true)
	countInflight = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	countDone     = lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
)

var (
	cursorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	selectedText = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	doneText     = lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Strikethrough(true)
)

var (
	badgeAgent      = lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Bold(true)
	badgeAgentGen   = lipgloss.NewStyle().Foreground(lipgloss.Color("141")).Bold(true)
	badgeAgentMCP   = lipgloss.NewStyle().Foreground(lipgloss.Color("117")).Bold(true)
	badgeDraft      = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	badgeDraftGen   = lipgloss.NewStyle().Foreground(lipgloss.Color("177")).Bold(true)
	badgeDraftMCP   = lipgloss.NewStyle().Foreground(lipgloss.Color("109")).Bold(true)
	badgeDone       = lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
	badgeDispatched = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	badgeNeedsInput = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
)

var (
	sectionName = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Bold(true)
	sectionRule = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
)

var (
	catReadyPill    = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("81")).Bold(true).Padding(0, 1)
	catManualPill   = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("252")).Bold(true).Padding(0, 1)
	catInflightPill = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("214")).Bold(true).Padding(0, 1)
	catDonePill     = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("242")).Bold(true).Padding(0, 1)
	catDoneSummary  = lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Italic(true)
)

var (
	helpFooter = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	footerKey  = lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Bold(true)
	footerDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

var (
	taskNum       = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	checkOpen     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	checkDone     = lipgloss.NewStyle().Foreground(lipgloss.Color("77"))
	fileNameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
)

var (
	helpKey     = lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Bold(true).Width(12).Align(lipgloss.Right)
	helpDesc    = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	helpSection = lipgloss.NewStyle().Foreground(lipgloss.Color("62")).Bold(true).MarginTop(1)
)

var (
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("77")).Italic(true)
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Italic(true)
)

var (
	settingsRepoName     = lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Bold(true)
	settingsRepoPath     = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	settingsRepoDesc     = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Italic(true)
	settingsLabel        = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Width(8)
	settingsValue        = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	settingsSelectedName = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	settingsDeleteWarn   = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
)
