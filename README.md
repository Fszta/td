<h1 align="center">td</h1>

<p align="center">
  <strong>Terminal daily planner</strong> — format, route, and dispatch your day with AI
</p>

<p align="center">
  <a href="#requirements"><img src="https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go 1.24+"></a>
  <img src="https://img.shields.io/badge/Terminal-TUI-4EAA25?style=flat-square&logo=gnometerminal&logoColor=white" alt="Terminal TUI">
  <img src="https://img.shields.io/badge/Markdown-tasks-000000?style=flat-square&logo=markdown&logoColor=white" alt="Markdown tasks">
  <img src="https://img.shields.io/badge/Claude-API-CC785C?style=flat-square" alt="Claude API">
  <img src="https://img.shields.io/badge/License-MIT-blue?style=flat-square" alt="MIT License">
</p>

<p align="center">
  Dump ideas and micro-tasks into a markdown file · Press <code>f</code> to structure · Press <code>d</code> to dispatch agents
</p>

<br>

> **Not** a team backlog (Linear, Jira, Asana). It's the step **before** you even open one — your personal morning inbox to capture what's on your mind and plan what you'll actually work on today.

---

## Quick start

| | Step | Command |
|---|------|---------|
| 1 | **Install** | `go install .` — or `make build && ./bin/td` |
| 2 | **API key** | `export ANTHROPIC_API_KEY=sk-ant-...` |
| 3 | **Repos** *(optional)* | `td scan ~/projects` |
| 4 | **Config** *(optional)* | `td config` — editor, `tasks_dir`, `[[repos]]`, `[[mcp]]` |
| 5 | **Run** | `td` |

<details>
<summary><strong>Optional: MCP target</strong> in <code>~/.config/td/config.toml</code></summary>

```toml
[[mcp]]
name = "notion"
description = "Create and update Notion pages and databases"
```

Same MCP servers must be configured in Claude Code (`claude mcp list`).

</details>

| Keys | Action |
|------|--------|
| `e` | Edit today's file |
| `f` | Format with Claude |
| `x` | Toggle done |
| `d` | Dispatch `@agent` task |

Dispatch needs the [Claude CLI](https://docs.anthropic.com/en/docs/claude-code) and MCP setup when routing to external tools.

---

## Contents

- [Walkthrough](#walkthrough)
- [Commands](#commands)
- [Keybindings](#keybindings)
- [File structure](#file-structure)
- [Configuration](#configuration)
- [Repo discovery](#repo-discovery)
- [Tags](#tags)
- [How formatting works](#how-formatting-works)
- [Carry-over](#carry-over)
- [Dispatch](#dispatch)
- [Requirements](#requirements)

---

## Walkthrough

```bash
td
```

On first run, your editor opens with today's blank file. Jot down whatever is on your mind — one line per task is enough; spelling and structure don't matter:

```
write rfc for new analytics repo, share with team
hourly airflow dag for dbt tag xyz
reply to alex on slack - kafka retention thread
terraform: add ecr repo for checkout-api
review sam's PR on web-app
draft ci for checkout-api, discuss approach with team first
```

Save and quit. Back in the TUI, press `f` to format — Claude turns each line into a clear checkbox and picks a tag when the task is agent-ready:

```markdown
## Tasks
- [ ] Write RFC for new analytics repo, share with team
- [ ] @agent:data-pipelines Create Airflow DAG: hourly dbt run, tag xyz
- [ ] Reply to Alex on Slack — Kafka retention thread
- [ ] @agent:infra-terraform Add ECR repository for checkout-api
- [ ] @draft Draft CI pipeline for checkout-api, discuss approach with team
- [ ] Review Sam's PR on web-app
```

The TUI renders each task with a right-aligned badge showing its category:

<pre>
╭──────────────────────────────────────────────────────────────╮
│  td                                 Wednesday, April 8 2026  │
│  ████████████░░░░░░░░░░░░░░░░░░░░░░  2/6 done               │
╰──────────────────────────────────────────────────────────────╯

  mine 2    agent 2    draft 1    done 1

  Tasks ────────────────────────────────────────────────────────
    1. [ ] Write RFC for analytics repo, share with team
  › 2. [ ] Create Airflow DAG: hourly dbt run   data-pipelines ⚡
    3. [x] Reply to Alex — Kafka retention                ✓
    4. [ ] Add ECR repo for checkout-api      infra-terraform ⚡
    5. [ ] Draft CI pipeline for checkout-api         draft ✎
    6. [ ] Review Sam's PR on web-app

  e edit  f format  x done  a tag  A assign  d dispatch(2)  Tab settings  ? help
</pre>

---

## Commands

| Command          | Description                              |
| ---------------- | ---------------------------------------- |
| `td`             | Open today's tasks in the TUI            |
| `td scan <dir>`  | Discover git repos and add them to config|
| `td config`      | Open config file in your editor          |
| `td help`        | Show help                                |

---

## Keybindings

Press `?` in the TUI for a full help overlay. Here's the summary:

### Navigation

| Key   | Action                    |
| ----- | ------------------------- |
| `j/k` | Navigate down / up        |
| `g/G` | Jump to first / last task |

### Tasks

| Key   | Action                                            |
| ----- | ------------------------------------------------- |
| `e`   | Open today's file in editor                       |
| `f`   | Format tasks with Claude (auto-classifies tags)   |
| `x`   | Toggle task done / undone                         |
| `D` / `Ctrl+D` | Delete task (confirm twice)                |
| `a`   | Cycle tag: none → agent → draft → none            |
| `A`   | Assign `@agent:repo` with filtered picker         |
| `d`   | Dispatch selected `@agent` task to repo agent     |

### General

| Key     | Action                    |
| ------- | ------------------------- |
| `Tab`   | Toggle settings view      |
| `c`     | Open config file in editor|
| `?`     | Toggle help overlay       |
| `q`     | Quit                      |

### Settings view

| Key     | Action             |
| ------- | ------------------ |
| `j/k`   | Navigate repos     |
| `s`     | Scan a directory for repos |
| `e`     | Edit config file   |
| `D`     | Delete selected repo (with confirmation) |
| `Tab`   | Back to tasks      |

---

## File structure

Tasks are organized by year and ISO week:

```
~/tasks/
  2026/
    W15/
      2026-04-07-mon.md
      2026-04-08-tue.md
    W16/
      2026-04-14-mon.md
      ...
```

Each file is plain markdown with checkboxes. Undone tasks from the previous day are automatically carried over into a `## Carried over` section.

---

## Configuration

Create `~/.config/td/config.toml` manually or run `td config`:

```toml
tasks_dir = "~/tasks"
editor = "nvim"
anthropic_api_key = "sk-ant-..."
anthropic_model = "claude-sonnet-4-6"

[[repos]]
name = "data-pipelines"
path = "~/projects/data-pipelines"
description = "Airflow DAGs and scheduled data jobs"

[[repos]]
name = "analytics"
path = "~/projects/analytics"
description = "dbt models and warehouse transformations"

[[repos]]
name = "infra-terraform"
path = "~/projects/infra-terraform"
description = "Terraform modules for cloud infrastructure"

# MCP targets — tasks whose deliverable is in an external tool (Notion, Linear, …).
# Dispatch runs headless claude in generic_workspace with an MCP-focused prompt.
# The same MCP servers must be configured in Claude Code (claude mcp list).

[[mcp]]
name = "notion"
description = "Create and update Notion pages and databases"

[[mcp]]
name = "linear"
description = "Create and update Linear issues and projects"
```

The API key can also be set via the `ANTHROPIC_API_KEY` environment variable (takes precedence over config file).

Manage repos from within the TUI by pressing `Tab` to open the settings view, where you can browse, scan, edit, and delete repositories.

---

## Repo discovery

You can scan for repos directly from the TUI: press `Tab` to open settings, then `s` to scan. You'll be prompted for a directory path, and the scan runs inline.

Alternatively, scan from the command line:

```bash
td scan ~/projects
```

This finds all git repos, sends their README and config files to Claude for analysis, and presents an interactive checklist:

```
  td scan — found 8 new repos

> [x] data-pipelines        Airflow DAGs and scheduled data jobs
  [x] analytics             dbt models and warehouse transformations
  [x] api-gateway           Go service: HTTP gateway and routing
  [ ] log-processor         Go service: log ingestion pipeline
  [x] infra-terraform       Terraform modules for cloud infrastructure
  ...

  Space/x: toggle   Enter: save to config   q: cancel
```

Selected repos are appended to your config. Already-configured repos are excluded automatically.

---

## Tags

`td` classifies tasks into three categories based on AI involvement:

| Tag               | Badge       | Meaning                                        |
| ----------------- | ----------- | ---------------------------------------------- |
| *(none)*          | *(no badge)* | **Mine** — human-only: write RFC, answer Slack, make decisions |
| `@agent`          | `agent ⚡`  | **Agent** — fully autonomous AI work: dispatch and review |
| `@agent:repo`     | `repo ⚡`   | **Agent** — same, with explicit target repo (skips routing) |
| `@agent:generic`  | `generic ✦` | **Agent** — scratchpad workspace (files, research, bootstraps) |
| `@agent:notion`   | `notion ◈` | **Agent** — MCP dispatch (primary deliverable in that tool) |
| `@draft`          | `draft ✎`  | **Draft** — AI produces a draft, human steers and iterates |
| `@draft:repo`     | `repo ✎`   | **Draft** — same, with explicit target repo |
| `@dispatched`     | `→ sent`   | **Dispatched** — sent to an agent, in progress |

When formatting with `f`, Claude automatically classifies each task:
- **Agent tasks**: well-scoped coding work an AI can complete independently
- **Draft tasks**: AI can produce a useful starting point, but you need to guide it
- **Mine tasks**: require judgment, communication, or context only you have

---

## How formatting works

When you press `f`, the raw file content is sent to Claude with instructions to:

- Structure tasks as markdown checkboxes
- Clarify wording while preserving intent
- Classify tasks into mine / agent / draft based on AI involvement level
- Auto-tag agent and draft tasks with `@agent:target` using configured repos, MCP names, or `generic`
- Keep carried-over tasks in their own section
- Never invent or remove tasks

---

## Carry-over

When `td` creates a new day's file, any unchecked tasks from the most recent previous file are placed under `## Carried over`. Tasks marked `[x]` are left behind.

---

## Dispatch

Press `d` on an `@agent` task to dispatch it:

- **With explicit target** (`@agent:data-pipelines`, `@agent:notion`, `@agent:generic`): goes directly to confirmation — no routing API call needed
- **Bare `@agent`**: Claude reads the task + your repos and MCP descriptions and picks the best match (repo, MCP, or generic)

Dispatch profiles:

| Target | Where it runs | Agent behavior |
|--------|----------------|----------------|
| Repo | Repository path | Git branch + task in that codebase |
| MCP (`notion`, `linear`, …) | `generic_workspace` | MCP-focused prompt; deliverable in the external tool |
| `generic` | `generic_workspace` | Files, research, bootstraps — no required MCP |

The confirmation panel shows the target and path. Press `Enter` to launch headless `claude` (stream-json log under `~/.config/td/trackers/`). The task tag updates to `@dispatched`. Press `r` to resume in iTerm2.

The dispatch flow requires:
- At least one `[[repos]]`, `[[mcp]]`, or use of `generic` routing
- `claude` CLI installed with MCP servers configured for the targets you use (`claude mcp list`)
- iTerm2 for resume (uses AppleScript to open a new tab)

---

## Requirements

| | Requirement |
|---|-------------|
| **Runtime** | Go 1.24+ |
| **AI** | Anthropic API key (`ANTHROPIC_API_KEY`) — format, routing, scan |
| **Dispatch** | [Claude CLI](https://docs.anthropic.com/en/docs/claude-code) + configured repos and/or MCP |
| **Resume** | macOS + iTerm2 (optional) |
