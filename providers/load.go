package providers

import (
	"fmt"
	"path/filepath"
	"strings"
)

func Load(path, prefix string) ([]map[string]any, error) {
	if rest, ok := strings.CutPrefix(path, "git:"); ok {
		return loadGit(rest, prefix)
	}
	switch ext := strings.ToLower(filepath.Ext(path)); ext {
	case ".json":
		return loadJSON(path, prefix)
	case ".yaml", ".yml":
		return loadYAML(path, prefix)
	default:
		return nil, fmt.Errorf("unsupported file extension %q: %s", ext, path)
	}
}
