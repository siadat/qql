package providers

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Load dispatches to the right provider based on ctx. If ctx.Provider is set
// (currently only `external:<script>` is recognized), it takes precedence over
// extension-based dispatch. Otherwise the extension of ctx.Source decides, with
// `git:` as a source-prefix shortcut for the git provider.
func Load(ctx Context) ([]map[string]any, error) {
	if rest, ok := strings.CutPrefix(ctx.Provider, "external:"); ok {
		return loadExternal(rest, ctx)
	}
	if ctx.Provider != "" {
		return nil, fmt.Errorf("unknown provider %q (expected external:<path>)", ctx.Provider)
	}
	if rest, ok := strings.CutPrefix(ctx.Source, "git:"); ok {
		return loadGit(rest, ctx.Prefix)
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
