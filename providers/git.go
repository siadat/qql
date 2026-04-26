package providers

import (
	"fmt"
	"os/exec"
	"strings"
)

func loadGit(repoPath, prefix string) ([]map[string]any, error) {
	if prefix != "" && prefix != "*" {
		return nil, fmt.Errorf("git provider does not support WITH prefix = %q", prefix)
	}
	if repoPath == "" {
		repoPath = "."
	}
	cmd := exec.Command("git", "-C", repoPath, "log", "--pretty=format:%H%x00%an%x00%ae%x00%aI%x00%s")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log in %s: %w", repoPath, err)
	}

	var rows []map[string]any
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x00", 5)
		if len(parts) != 5 {
			return nil, fmt.Errorf("malformed git log line: %q", line)
		}
		rows = append(rows, map[string]any{
			"commit":  parts[0],
			"author":  parts[1],
			"email":   parts[2],
			"time":    parts[3],
			"subject": parts[4],
		})
	}
	return rows, nil
}
