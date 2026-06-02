package main

import (
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"

	"td/internal/config"
	"td/internal/scan"
	"td/internal/taskfile"
	"td/internal/tui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "scan":
			runScan(cfg)
		case "config":
			runConfig(cfg)
		case "help", "--help", "-h":
			printHelp()
		default:
			fmt.Fprintf(os.Stderr, "unknown command: %s\nrun 'td help' for usage\n", os.Args[1])
			os.Exit(1)
		}
		return
	}

	runTUI(cfg)
}

func runTUI(cfg *config.Config) {
	filePath, created, err := taskfile.EnsureToday(cfg.TasksDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	tasks, err := taskfile.Parse(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	m := tui.NewModel(cfg, filePath, tasks, created)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runScan(cfg *config.Config) {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: td scan <directory>")
		os.Exit(1)
	}

	if cfg.AnthropicAPIKey == "" {
		fmt.Fprintln(os.Stderr, "error: set ANTHROPIC_API_KEY to use td scan")
		os.Exit(1)
	}

	dir := os.Args[2]
	existing := config.ExistingRepoNames(cfg)

	m := scan.NewScanModel(dir, cfg.AnthropicAPIKey, cfg.AnthropicModel, existing)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runConfig(cfg *config.Config) {
	configPath, err := config.EnsureConfigFile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	c := exec.Command(cfg.Editor, configPath)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Print(`td — daily planner with AI formatting, routing, and dispatch

Usage:
  td              Open today's tasks in the TUI
  td scan <dir>   Discover git repos and add them to config
  td config       Open config file in your editor
  td help         Show this help

Keybindings (in TUI):
  e       Open today's file in editor
  f       Format tasks with Claude
  x       Toggle task done / undone
  D       Delete task (press twice); Ctrl+D same
  a       Cycle tag: none → agent → draft → none
  A       Assign @agent:repo with picker
  d       Dispatch @agent task to repo
  w       Watch session log (in-flight or selected dispatched task)
  r       Resume in iTerm (two presses while session is running)
  h       Toggle sessions sidebar (past agent runs)
  Tab     Toggle settings view (scan, manage repos)
  c       Open config file
  ?       Toggle help overlay
  j/k     Navigate down / up
  g/G     Jump to first / last task
  q       Quit
`)
}
