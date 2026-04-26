package readers

import (
	"fmt"
	"os/exec"
	"strings"
)

func loadGit(repoPath string) (any, error) {
	if repoPath == "" {
		repoPath = "."
	}
	cmd := exec.Command("git", "-C", repoPath, "log", "--pretty=format:%H%x00%an%x00%ae%x00%aI%x00%s")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log in %s: %w", repoPath, err)
	}

	commits := map[string]any{}
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x00", 5)
		if len(parts) != 5 {
			return nil, fmt.Errorf("malformed git log line: %q", line)
		}
		commits[parts[0]] = map[string]any{
			"author":  parts[1],
			"email":   parts[2],
			"time":    parts[3],
			"subject": parts[4],
		}
	}
	return commits, nil
}
