package taskfile

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func EnsureToday(tasksDir string) (string, bool, error) {
	now := time.Now()
	_, week := now.ISOWeek()

	dir := filepath.Join(
		tasksDir,
		now.Format("2006"),
		fmt.Sprintf("W%02d", week),
	)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", false, fmt.Errorf("creating directory %s: %w", dir, err)
	}

	fileName := fmt.Sprintf("%s-%s.md",
		now.Format("2006-01-02"),
		strings.ToLower(now.Format("Mon")),
	)
	filePath := filepath.Join(dir, fileName)

	if _, err := os.Stat(filePath); err == nil {
		return filePath, false, nil
	}

	carried := findCarriedTasks(tasksDir, filePath)
	content := generateTemplate(now, carried)

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return "", false, err
	}

	return filePath, true, nil
}

func findCarriedTasks(tasksDir, todayPath string) []Task {
	prev := findPreviousFile(tasksDir, todayPath)
	if prev == "" {
		return nil
	}

	tasks, err := Parse(prev)
	if err != nil {
		return nil
	}

	var carried []Task
	for _, t := range tasks {
		if t.Status == Open {
			carried = append(carried, t)
		}
	}
	return carried
}

func findPreviousFile(tasksDir, todayPath string) string {
	var files []string

	if err := filepath.WalkDir(tasksDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".md") && path != todayPath {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		return ""
	}

	if len(files) == 0 {
		return ""
	}

	sort.Strings(files)
	return files[len(files)-1]
}

func generateTemplate(date time.Time, carried []Task) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n", date.Format("Monday, January 2 2006"))

	if len(carried) > 0 {
		b.WriteString("\n## Carried over\n")
		for _, t := range carried {
			t.Status = Open
			b.WriteString(FormatTaskLine(t) + "\n")
		}
	}

	b.WriteString("\n## Tasks\n\n")
	b.WriteString("\n## Notes\n\n")

	return b.String()
}
