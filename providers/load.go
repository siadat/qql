package providers

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Load dispatches to the right provider based on ctx, returning one or more
// row groups. YAML files with multiple documents (`---`) yield one group per
// document; every other source returns a single group. Callers render one
// table per group.
//
// When ctx.Provider is non-empty it takes precedence: `external:<script>`
// runs a user-supplied script and `git:<repo>` reads commit rows from a
// repository. With no provider set, the extension of ctx.Source decides
// between the YAML and JSON readers.
func Load(ctx Context) ([][]map[string]any, error) {
	if rest, ok := strings.CutPrefix(ctx.Provider, "external:"); ok {
		rows, err := loadExternal(rest, ctx)
		if err != nil {
			return nil, err
		}
		return [][]map[string]any{rows}, nil
	}
	if rest, ok := strings.CutPrefix(ctx.Provider, "git:"); ok {
		rows, err := loadGit(rest)
		if err != nil {
			return nil, err
		}
		return [][]map[string]any{rows}, nil
	}
	if ctx.Provider != "" {
		return nil, fmt.Errorf("unknown provider %q (expected external:<path> or git:<repo>)", ctx.Provider)
	}
	switch ext := strings.ToLower(filepath.Ext(ctx.Source)); ext {
	case ".json":
		rows, err := loadJSON(ctx.Source)
		if err != nil {
			return nil, err
		}
		return [][]map[string]any{rows}, nil
	case ".yaml", ".yml":
		return loadYAML(ctx.Source)
	default:
		return nil, fmt.Errorf("unsupported file extension %q: %s", ext, ctx.Source)
	}
}
