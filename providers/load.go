package providers

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Load dispatches to the right provider based on ctx. When ctx.Provider is
// non-empty it takes precedence: `external:<script>` runs a user-supplied
// script and `git:<repo>` reads commit rows from a repository. With no
// provider set, the extension of ctx.Source decides between the YAML and JSON
// readers.
func Load(ctx Context) ([]map[string]any, error) {
	if rest, ok := strings.CutPrefix(ctx.Provider, "external:"); ok {
		return loadExternal(rest, ctx)
	}
	if rest, ok := strings.CutPrefix(ctx.Provider, "git:"); ok {
		return loadGit(rest, ctx.Prefix)
	}
	if ctx.Provider != "" {
		return nil, fmt.Errorf("unknown provider %q (expected external:<path> or git:<repo>)", ctx.Provider)
	}
	switch ext := strings.ToLower(filepath.Ext(ctx.Source)); ext {
	case ".json":
		return loadJSON(ctx.Source, ctx.Prefix)
	case ".yaml", ".yml":
		return loadYAML(ctx.Source, ctx.Prefix)
	default:
		return nil, fmt.Errorf("unsupported file extension %q: %s", ext, ctx.Source)
	}
}
