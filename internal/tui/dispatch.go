package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"td/internal/config"
	"td/internal/tracker"
)

func launchHeadless(store *tracker.Store, repoPath, repoName, taskText string, mcps []config.MCP) (*tracker.Task, error) {
	generic := repoName == config.GenericRepoName
	mcp := findMCP(mcps, repoName)
	isMCP := mcp != nil

	if generic || isMCP {
		if repoPath == "" {
			return nil, fmt.Errorf("workspace path is empty — set generic_workspace in config")
		}
		if err := os.MkdirAll(repoPath, 0o755); err != nil {
			return nil, fmt.Errorf("creating workspace: %w", err)
		}
	}

	id := tracker.TaskID(repoName, taskText)
	branch := tracker.BranchName(taskText)
	logPath := store.LogPath(id)

	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, err
	}

	prompt := buildDispatchPrompt(generic, isMCP, repoName, branch, taskText)

	cmd := exec.Command("claude",
		"--print",
		"--output-format", "stream-json",
		"--verbose",
		"--dangerously-skip-permissions",
		"--name", "td: "+truncateStr(taskText, 60),
		"--max-budget-usd", "5",
		prompt,
	)
	cmd.Dir = repoPath
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return nil, err
	}

	go func() {
		_ = cmd.Wait()
		logFile.Close()
	}()

	t := &tracker.Task{
		ID:        id,
		TaskText:  taskText,
		RepoName:  repoName,
		RepoPath:  repoPath,
		Branch:    branch,
		PID:       cmd.Process.Pid,
		StartedAt: time.Now(),
		Status:    tracker.StatusRunning,
	}

	if err := store.Register(t); err != nil {
		return t, err
	}
	return t, nil
}

const genericPromptTemplate = `You are an autonomous agent running in a personal scratchpad directory (your current working directory).

Your job: COMPLETE THE TASK end-to-end. Produce concrete, useful deliverables — not placeholders, not just a directory or empty file.

Guidance by task type:
- Research / framing tasks: use web search and web fetch tools to gather context (regulations, prior art, market info, naming conventions) BEFORE drafting. Cite sources in your output where relevant.
- Project bootstraps: create the directory structure AND the initial meaningful content (README with vision/scope, key source skeletons, config files).
- Document tasks (SQL, markdown, RFC, design doc, plan): produce the full document, structured with clear sections that address every part of the task.

Create files and subdirectories as needed. Pick sensible names and a clear structure. Branch creation and git operations are optional unless you are scaffolding a new project that should live in its own repo.

DO NOT stop after a single mkdir, empty file, or one-line placeholder. Keep working until the deliverable substantively addresses every part of the task below.

Task:
%s`

const mcpPromptTemplate = `You are an autonomous agent with MCP tools configured, including access to %s.

Your job: COMPLETE THE TASK end-to-end using the %s MCP tools as the primary way to deliver the outcome. Create or update the real artifact in %s (page, issue, database entry, etc.) — do not substitute a local markdown file unless the task explicitly asks for a draft on disk.

Guidance:
- Use the relevant MCP tools first. If authentication or permissions fail, report clearly what blocked you.
- When done, summarize what you created or changed and include links or IDs from the tool responses.
- You may use web search or local files only as supporting context, not as the main deliverable.
- Do NOT create a git branch unless the task also requires code changes in this directory.

DO NOT stop after planning or a placeholder. Keep working until the external artifact exists and the task is addressed.

Task:
%s`

func findMCP(mcps []config.MCP, name string) *config.MCP {
	for i := range mcps {
		if strings.EqualFold(mcps[i].Name, name) {
			return &mcps[i]
		}
	}
	return nil
}

func buildDispatchPrompt(generic, isMCP bool, mcpName, branch, taskText string) string {
	if isMCP {
		return fmt.Sprintf(mcpPromptTemplate, mcpName, mcpName, mcpName, taskText)
	}
	if generic {
		return fmt.Sprintf(genericPromptTemplate, taskText)
	}
	return fmt.Sprintf(
		"First, create and checkout a new git branch named %q from the current branch. Then: %s",
		branch, taskText,
	)
}

func resumeInITerm(t *tracker.Task) error {
	dir := t.RepoPath
	shellCmd := fmt.Sprintf("cd %s && claude --resume %s", shellQuote(dir), shellQuote(t.SessionID))
	asCmd := escapeAppleScript(shellCmd)

	script := fmt.Sprintf(`tell application "iTerm2"
	tell current window
		tell current session
			set newSession to (split vertically with default profile)
		end tell
		tell newSession
			write text "%s"
		end tell
	end tell
end tell`, asCmd)

	cmd := exec.Command("osascript", "-e", script)
	return cmd.Run()
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
