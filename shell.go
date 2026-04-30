package main

import (
	"fmt"
	"io"
	"strings"
)

// printShell writes each row as space-separated `col="value"` pairs, one
// row per line. Values are double-quoted with escaping for the four
// characters that retain special meaning inside bash/sh double quotes
// (`\`, `"`, `$`, backtick) so a line can be pasted into a shell as a
// sequence of variable assignments. Column names are emitted verbatim,
// so dot-paths like `cfg.timeout` still appear as `cfg.timeout="..."` —
// not a valid POSIX variable name, but unambiguous and easy to grep.
func printShell(w io.Writer, rows []row, selected []string, excluded []string) {
	cols := resolveCols(rows, selected, excluded)
	var sb strings.Builder
	for _, r := range rows {
		sb.Reset()
		for i, c := range cols {
			if i > 0 {
				sb.WriteByte(' ')
			}
			sb.WriteString(c)
			sb.WriteByte('=')
			sb.WriteString(shellQuote(r[c]))
		}
		fmt.Fprintln(w, sb.String())
	}
}

func shellQuote(v any) string {
	if v == nil {
		return `""`
	}
	s := fmt.Sprintf("%v", v)
	var sb strings.Builder
	sb.Grow(len(s) + 2)
	sb.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\', '"', '$', '`':
			sb.WriteByte('\\')
		}
		sb.WriteRune(r)
	}
	sb.WriteByte('"')
	return sb.String()
}
