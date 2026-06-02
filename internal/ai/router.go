package ai

import (
	"encoding/json"
	"fmt"
	"strings"

	"td/internal/config"
)

const routerSystemPrompt = `You are a task router. Given a task description, repositories, and MCP integrations, determine:

1. Is the task actionable? Does it have enough context for an AI agent to start working
   without asking clarifying questions? A task needs a clear action and enough detail
   to produce a concrete output.

2. If actionable, where should it run?

Respond with a JSON object (no markdown fences, no explanation):

If it maps to a configured repo (code, infra, data pipelines in that codebase):
{"repo": "repo-name", "actionable": true}

If the primary deliverable is in an external system via a configured MCP
(Notion page, Linear issue, etc.):
{"mcp": "mcp-name", "actionable": true}

If actionable but it does NOT fit any repo or MCP (new project bootstraps,
standalone docs/SQL on disk, one-off scripts, exploration work, etc.):
{"generic": true, "actionable": true}

If NOT actionable (too vague, missing context):
{"actionable": false, "missing": "brief description of what's missing"}

Routing priority: prefer repo when the task is mainly code/config in that repo.
Prefer MCP when the task is mainly creating/updating records in that external tool.
Use generic only when neither applies. Never invent repo or MCP names.

Examples:
- "create Airflow DAG for hourly dbt run" with repo "data-pipelines" → {"repo": "data-pipelines", "actionable": true}
- "create a Notion page for Q2 planning" with mcp "notion" → {"mcp": "notion", "actionable": true}
- "file Linear issue for broken DAG alert" with mcp "linear" → {"mcp": "linear", "actionable": true}
- "write a SQL procedure to migrate historical data" with no matching repo → {"generic": true, "actionable": true}
- "check if we got updates" → {"actionable": false, "missing": "which system, what kind of updates, what to do with them"}`

type RouteResult struct {
	Repo       string `json:"repo"`
	MCP        string `json:"mcp"`
	Generic    bool   `json:"generic"`
	Actionable bool   `json:"actionable"`
	Missing    string `json:"missing,omitempty"`
}

func (c *Client) Route(taskText string, repos []config.Repo, mcps []config.MCP) (*RouteResult, error) {
	var targets strings.Builder
	if len(repos) > 0 {
		targets.WriteString("Repositories:\n")
		for _, r := range repos {
			fmt.Fprintf(&targets, "- %s: %s\n", r.Name, r.Description)
		}
	}
	if len(mcps) > 0 {
		if targets.Len() > 0 {
			targets.WriteString("\n")
		}
		targets.WriteString("MCP integrations (external tools):\n")
		for _, m := range mcps {
			fmt.Fprintf(&targets, "- %s: %s\n", m.Name, m.Description)
		}
	}

	prompt := fmt.Sprintf("Task: %s\n\n%s", taskText, targets.String())
	result, err := c.Complete(routerSystemPrompt, prompt)
	if err != nil {
		return nil, err
	}

	result = strings.TrimSpace(result)
	result = strings.TrimPrefix(result, "```json")
	result = strings.TrimPrefix(result, "```")
	result = strings.TrimSuffix(result, "```")
	result = strings.TrimSpace(result)

	var route RouteResult
	if err := json.Unmarshal([]byte(result), &route); err != nil {
		return &RouteResult{Actionable: false, Missing: "routing response was not valid JSON"}, nil
	}

	// Normalise: a literal "generic" repo name is the generic workspace.
	if strings.EqualFold(route.Repo, "generic") {
		route.Repo = ""
		route.Generic = true
	}

	// MCP takes precedence over accidental repo field with same name.
	if route.MCP != "" {
		route.Repo = ""
		route.Generic = false
	}

	return &route, nil
}
