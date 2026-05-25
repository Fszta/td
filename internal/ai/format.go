package ai

import (
	"fmt"
	"strings"

	"td/internal/config"
)

const formatSystemPrompt = `You are a task formatter for a developer.

Given raw, messy notes about today's work, produce a clean markdown task file.

Rules:
- Start with a heading: # {weekday}, {month} {day} {year}
- Add a ## Tasks section with one checkbox per distinct task: - [ ] description
- Clarify wording but preserve the original intent. Do not invent or remove tasks.
- Split compound tasks into sub-items only when they involve truly separate actions.
- Each task has a short TITLE on the checkbox line and optionally a DESCRIPTION
  as indented continuation lines below. The title is a concise summary. The description
  preserves all important context, details, file paths, technical specs, mappings, etc.
  that an AI agent or the human will need. Example:
  - [ ] @draft:analytics Draft migration approach — source/target DDL guidance
    Source DDL: ~/projects/analytics/ddl/migration_sources.md
    Create SQL wrapped in BEGIN/ROLLBACK (COMMIT commented). Only insert rows
    from source not in target (PK = ID). Mapping: operation_type = "snapshot",
    committed_at from _cdc_updated_at, emitted_at from _airbyte_emitted_at
  NEVER discard context from the user's notes. If it has details, put them in
  the description. A task without its context is useless.
- Classify each task into one of three categories using tags placed at the START
  of the task text, right after the checkbox (e.g. "- [ ] @agent:repo Task text"):

  1. @agent — fully autonomous AI tasks. ONLY use this when ALL of the following are true:
     - The task has a clear, specific action (create, add, write, scaffold, fix...)
     - The target system/repo/component is obvious or stated
     - An AI agent could complete it WITHOUT asking clarifying questions
     - The expected output is well-defined (a PR, a file, a config...)
     Examples: "create Airflow DAG for hourly dbt run with tag xyz",
     "add Terraform ECR repo for checkout-api", "scaffold dbt staging model for payments"

  2. @draft — collaborative AI tasks. Use when AI can produce a useful starting point
     but the task needs human judgment, iteration, or is underspecified:
     - Design decisions are required
     - Team input or discussion is mentioned
     - The scope is broad or ambiguous
     Examples: "draft CI pipeline for checkout-api, discuss with team",
     "prototype migration plan for Kafka cluster", "outline test scenarios for auth flow"

  3. No tag — human-only tasks, OR tasks that are too vague for AI to act on.
     If a task lacks enough context for an agent to know what to do, leave it untagged
     even if it sounds technical. The human needs to refine it first.
     Examples: "write RFC for analytics repo", "reply to Alex on Slack",
     "review Sam's PR on web-app", "check if vendor sent the update" (too vague — which system? what action?)

- When in doubt, do NOT tag. It's better to leave a task untagged than to
  send an agent on a vague mission. The user can always add a tag manually.
- Preserve any existing [x] done states and @dispatched tags.
- If there is a ## Carried over section, keep it separate from ## Tasks.
- Add an empty ## Notes section at the end.
- Output ONLY the markdown content. No explanation, no code fences.`

const repoTargetingAddendum = `

When adding @agent or @draft tags, ALWAYS include a target using @agent:target
or @draft:target. Do NOT use bare @agent or @draft without a target.

The target MUST be one of:

1. The name of a configured repository, when the task clearly belongs to it:
%s

2. The reserved name "generic", when the task is actionable but has no repo
   home — for example: bootstrapping a new project, drafting a SQL procedure
   or migration plan as a standalone file, writing a one-off script, producing
   a markdown doc or RFC, exploration work, etc. Examples:
   - "bootstrap a new app for X" → @agent:generic
   - "write a SQL procedure to migrate historical data to the warehouse" → @agent:generic
   - "draft an RFC for the new dbt repo layout" → @draft:generic

3. A configured MCP name, when the deliverable lives in that external tool
   (Notion page, Linear issue, etc.):
%s

NEVER invent a target name that is not in the lists above. If the task does not
fit any configured repo, MCP, or generic workspace, leave it untagged so the
human can refine it.`

func (c *Client) Format(content string, repos []config.Repo, mcps []config.MCP) (string, error) {
	system := formatSystemPrompt

	if len(repos) > 0 || len(mcps) > 0 {
		var repoList strings.Builder
		for _, r := range repos {
			fmt.Fprintf(&repoList, "- %s: %s\n", r.Name, r.Description)
		}
		var mcpList strings.Builder
		for _, m := range mcps {
			fmt.Fprintf(&mcpList, "- %s: %s\n", m.Name, m.Description)
		}
		if len(mcps) == 0 {
			mcpList.WriteString("(none configured)\n")
		}
		system += fmt.Sprintf(repoTargetingAddendum, repoList.String(), mcpList.String())
	}

	return c.Complete(system, content)
}
