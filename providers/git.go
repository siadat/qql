package providers

import (
	"fmt"
	"os/exec"
	"strings"
)

// loadGit runs `git log` once and emits one row per commit. The columns
// (in order) are:
//
//	commit          %H   full commit hash
//	author_name     %an
//	author_email    %ae
//	author_time     %aI  ISO 8601 (sorts lexicographically)
//	committer_name  %cn
//	committer_email %ce
//	committer_time  %cI
//	parents         %P   space-separated parent hashes ("" for the root)
//	refs            %D   ref names (full form, e.g. "refs/heads/main")
//	subject         %s   first line of the commit message
//	is_merge             derived: parents has more than one hash
//
// Fields are NUL-separated and rows are newline-separated. This stays safe
// because %s is single-line by git's contract; if you ever add %b or %B
// (commit body / full message), switch to `git log -z` and pick a different
// intra-record separator since bodies contain literal newlines.
//
// --decorate=full is required because git only auto-decorates when stdout is
// a tty, and cmd.Output() captures into a pipe. Without it, %D is empty for
// every commit.
func loadGit(repoPath string) ([]map[string]any, error) {
	if repoPath == "" {
		repoPath = "."
	}
	cmd := exec.Command("git", "-C", repoPath,
		"log", "--decorate=full",
		"--pretty=format:%H%x00%an%x00%ae%x00%aI%x00%cn%x00%ce%x00%cI%x00%P%x00%D%x00%s")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log in %s: %w", repoPath, err)
	}

	var rows []map[string]any
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x00", 10)
		if len(parts) != 10 {
			return nil, fmt.Errorf("malformed git log line: %q", line)
		}
		parents := parts[7]
		rows = append(rows, map[string]any{
			"commit":          parts[0],
			"author_name":     parts[1],
			"author_email":    parts[2],
			"author_time":     parts[3],
			"committer_name":  parts[4],
			"committer_email": parts[5],
			"committer_time":  parts[6],
			"parents":         parents,
			"refs":            parts[8],
			"subject":         parts[9],
			"is_merge":        len(strings.Fields(parents)) > 1,
		})
	}
	return rows, nil
}
