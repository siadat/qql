package providers

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// externalRequest is the JSON payload qql writes to the script's stdin.
//
// `version` lets us evolve the protocol without silently breaking scripts;
// every other field is a hint. The script may filter, ignore, or reparse them
// however it likes — qql re-applies the parsed WHERE/ORDER BY/SELECT to the
// returned rows regardless.
type externalRequest struct {
	Version int         `json:"version"`
	Source  string      `json:"source,omitempty"`
	Files   []string    `json:"files,omitempty"`
	Prefix  string      `json:"prefix,omitempty"`
	Select  []string    `json:"select,omitempty"`
	Where   string      `json:"where,omitempty"`
	OrderBy []OrderTerm `json:"order_by,omitempty"`
	// Limit is a *int so absence ("no LIMIT clause") and 0 ("LIMIT 0") are
	// distinguishable on the wire. omitempty drops nil, leaves 0 in. Offset
	// is a plain int because OFFSET 0 is semantically a no-op, so omitempty's
	// zero-equals-absent collapse is the right behavior.
	Limit  *int `json:"limit,omitempty"`
	Offset int  `json:"offset,omitempty"`
}

// loadExternal execs scriptPath, sends the request payload on stdin, and
// reads JSONL rows from stdout. Stderr passes through to the user's terminal
// untouched; a non-zero exit causes loadExternal to return an error and
// discard whatever rows were read. Empty lines and `#`-prefixed comment lines
// in stdout are skipped to keep the format hand-debug friendly; malformed JSON
// lines are logged to stderr and skipped (so a noisy script doesn't take down
// a whole query).
func loadExternal(scriptPath string, ctx Context) ([]map[string]any, error) {
	var limit *int
	if ctx.Limit >= 0 {
		l := ctx.Limit
		limit = &l
	}
	payload, err := json.Marshal(externalRequest{
		Version: 1,
		Source:  ctx.Source,
		Files:   ctx.Files,
		Prefix:  ctx.Prefix,
		Select:  ctx.Select,
		Where:   ctx.Where,
		OrderBy: ctx.OrderBy,
		Limit:   limit,
		Offset:  ctx.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("encode external provider payload: %w", err)
	}

	cmd := exec.Command(scriptPath)
	cmd.Stdin = bytes.NewReader(payload)
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("external %s: stdout pipe: %w", scriptPath, err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("external %s: start: %w", scriptPath, err)
	}

	rows, scanErr := readExternalRows(stdout, ctx.Prefix)

	if waitErr := cmd.Wait(); waitErr != nil {
		return nil, fmt.Errorf("external %s exited with error: %w", scriptPath, waitErr)
	}
	if scanErr != nil {
		return nil, fmt.Errorf("external %s: read stdout: %w", scriptPath, scanErr)
	}
	return rows, nil
}

// readExternalRows decodes the JSONL stream from a script's stdout. Each
// non-blank, non-`#` line must be an envelope of the form
// {"type": "row"|"tree", "value": ...}:
//
//   - "row" passes value through as a single row.
//   - "tree" feeds value to rowsFromTree(value, prefix), expanding it the
//     same way the built-in YAML/JSON loaders do.
//
// Lines that don't match either shape are logged to stderr and skipped, so
// one bad line can't take down the whole query.
func readExternalRows(r io.Reader, prefix string) ([]map[string]any, error) {
	var rows []map[string]any
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		dec := json.NewDecoder(strings.NewReader(line))
		dec.UseNumber()
		var env map[string]any
		if err := dec.Decode(&env); err != nil {
			fmt.Fprintf(os.Stderr, "qql: skipping malformed external line %q: %v\n", line, err)
			continue
		}
		kind, _ := env["type"].(string)
		value, hasValue := env["value"]
		if !hasValue || (kind != "row" && kind != "tree") {
			fmt.Fprintf(os.Stderr, "qql: skipping external line %q: expected {\"type\":\"row\"|\"tree\",\"value\":...}\n", line)
			continue
		}
		switch kind {
		case "row":
			row, ok := value.(map[string]any)
			if !ok {
				fmt.Fprintf(os.Stderr, "qql: skipping external row line %q: value must be an object\n", line)
				continue
			}
			rows = append(rows, row)
		case "tree":
			expanded, err := rowsFromTree(value, prefix)
			if err != nil {
				fmt.Fprintf(os.Stderr, "qql: skipping external tree line %q: %v\n", line, err)
				continue
			}
			rows = append(rows, expanded...)
		}
	}
	return rows, scanner.Err()
}
