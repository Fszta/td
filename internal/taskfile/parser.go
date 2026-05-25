package taskfile

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

type Status int

const (
	Open Status = iota
	Done
)

type Tag string

const (
	TagNone       Tag = ""
	TagAgent      Tag = "@agent"
	TagDraft      Tag = "@draft"
	TagDispatched Tag = "@dispatched"
)

type Task struct {
	Line        int
	EndLine     int
	Status      Status
	Tag         Tag
	Target      string
	Text        string
	Description string
	Section     string
}

// FullText returns the title followed by the description, suitable for
// passing as context to an AI agent or displaying in full.
func (t Task) FullText() string {
	if t.Description == "" {
		return t.Text
	}
	return t.Text + "\n" + t.Description
}

var taskRe = regexp.MustCompile(`^-\s*\[([ xX])\]\s*(.*)$`)

func Parse(path string) ([]Task, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	var tasks []Task
	section := ""

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "## ") {
			section = strings.TrimPrefix(trimmed, "## ")
			continue
		}

		m := taskRe.FindStringSubmatch(trimmed)
		if m != nil {
			status := Open
			if m[1] == "x" || m[1] == "X" {
				status = Done
			}

			text := m[2]
			tag, target, text := extractTag(text)

			tasks = append(tasks, Task{
				Line:    i,
				EndLine: i,
				Status:  status,
				Tag:     tag,
				Target:  target,
				Text:    text,
				Section: section,
			})
			continue
		}

		// Indented continuation lines belong to the previous task
		if len(tasks) > 0 && trimmed != "" && (strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "\t")) {
			last := &tasks[len(tasks)-1]
			if last.Description != "" {
				last.Description += "\n"
			}
			last.Description += trimmed
			last.EndLine = i
		}
	}

	return tasks, nil
}

var tagWithTargetRe = regexp.MustCompile(`@(agent|draft|dispatched)(?::(\S+))?`)

func extractTag(text string) (Tag, string, string) {
	switch {
	case strings.HasPrefix(text, "@agent:"):
		rest := strings.TrimPrefix(text, "@agent:")
		target, body := splitTargetText(rest)
		return TagAgent, target, body
	case strings.HasPrefix(text, "@draft:"):
		rest := strings.TrimPrefix(text, "@draft:")
		target, body := splitTargetText(rest)
		return TagDraft, target, body
	case strings.HasPrefix(text, "@agent "):
		return TagAgent, "", strings.TrimPrefix(text, "@agent ")
	case strings.HasPrefix(text, "@draft "):
		return TagDraft, "", strings.TrimPrefix(text, "@draft ")
	case strings.HasPrefix(text, "@dispatched "):
		return TagDispatched, "", strings.TrimPrefix(text, "@dispatched ")
	}

	loc := tagWithTargetRe.FindStringSubmatchIndex(text)
	if loc == nil {
		return TagNone, "", text
	}

	tagName := text[loc[2]:loc[3]]
	target := ""
	if loc[4] != -1 {
		target = text[loc[4]:loc[5]]
	}

	body := strings.TrimSpace(text[:loc[0]])
	if trailing := strings.TrimSpace(text[loc[1]:]); trailing != "" {
		body += " " + trailing
	}

	switch tagName {
	case "agent":
		return TagAgent, target, body
	case "draft":
		return TagDraft, target, body
	case "dispatched":
		return TagDispatched, "", body
	}

	return TagNone, "", text
}

func splitTargetText(s string) (target, text string) {
	if idx := strings.Index(s, " "); idx != -1 {
		return s[:idx], s[idx+1:]
	}
	return s, ""
}

func FormatTaskLine(t Task) string {
	check := " "
	if t.Status == Done {
		check = "x"
	}

	tag := ""
	switch {
	case t.Tag == TagAgent && t.Target != "":
		tag = "@agent:" + t.Target + " "
	case t.Tag == TagDraft && t.Target != "":
		tag = "@draft:" + t.Target + " "
	case t.Tag != TagNone:
		tag = string(t.Tag) + " "
	}

	line := fmt.Sprintf("- [%s] %s%s", check, tag, t.Text)
	if t.Description != "" {
		for _, dl := range strings.Split(t.Description, "\n") {
			line += "\n  " + dl
		}
	}
	return line
}

func UpdateTask(path string, t Task) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	if t.Line < 0 || t.Line >= len(lines) {
		return fmt.Errorf("line %d out of range", t.Line)
	}

	newLines := strings.Split(FormatTaskLine(t), "\n")

	end := t.EndLine + 1
	if end > len(lines) {
		end = len(lines)
	}
	result := make([]string, 0, len(lines)-(end-t.Line)+len(newLines))
	result = append(result, lines[:t.Line]...)
	result = append(result, newLines...)
	result = append(result, lines[end:]...)

	return os.WriteFile(path, []byte(strings.Join(result, "\n")), 0644)
}

func DeleteTask(path string, t Task) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	if t.Line < 0 || t.Line >= len(lines) {
		return fmt.Errorf("line %d out of range", t.Line)
	}

	end := t.EndLine + 1
	if end > len(lines) {
		end = len(lines)
	}

	result := make([]string, 0, len(lines)-(end-t.Line))
	result = append(result, lines[:t.Line]...)
	result = append(result, lines[end:]...)

	return os.WriteFile(path, []byte(strings.Join(result, "\n")), 0644)
}

func WriteContent(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

func ReadContent(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
