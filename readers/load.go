package readers

import (
	"fmt"
	"path/filepath"
	"strings"
)

func Load(path string) (any, error) {
	switch ext := strings.ToLower(filepath.Ext(path)); ext {
	case ".json":
		return loadJSON(path)
	case ".yaml", ".yml":
		return loadYAML(path)
	default:
		return nil, fmt.Errorf("unsupported file extension %q: %s", ext, path)
	}
}
