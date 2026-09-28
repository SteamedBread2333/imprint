package index

import (
	"path/filepath"
	"strings"
)

// NormalizeDocPath canonicalizes a workspace-relative document path for matching.
func NormalizeDocPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.TrimPrefix(p, "./")
	return strings.Trim(p, "/")
}

// AllowPathSet builds a lookup set from grep or find path filters. Empty input returns nil (allow all).
func AllowPathSet(paths []string) map[string]struct{} {
	if len(paths) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		n := NormalizeDocPath(p)
		if n != "" {
			set[n] = struct{}{}
		}
	}
	return set
}

func chunkMatchesPathSet(chunkPath string, allow map[string]struct{}) bool {
	if allow == nil {
		return true
	}
	cp := NormalizeDocPath(chunkPath)
	if _, ok := allow[cp]; ok {
		return true
	}
	cb := filepath.ToSlash(filepath.Base(cp))
	for p := range allow {
		ap := NormalizeDocPath(p)
		if ap == cp {
			return true
		}
		if strings.HasSuffix(ap, "/"+cp) || strings.HasSuffix(cp, "/"+ap) {
			return true
		}
		ab := filepath.ToSlash(filepath.Base(ap))
		if ab == cb && (strings.HasSuffix(cp, ap) || strings.HasSuffix(ap, cp)) {
			return true
		}
	}
	return false
}

// FilterChunksByPaths keeps chunks whose path matches any normalized entry in paths.
// Unknown or non-index paths in paths are simply never matched.
func FilterChunksByPaths(chunks []Chunk, paths []string) []Chunk {
	allow := AllowPathSet(paths)
	if allow == nil {
		return chunks
	}
	out := make([]Chunk, 0, len(chunks))
	for _, c := range chunks {
		if chunkMatchesPathSet(c.Path, allow) {
			out = append(out, c)
		}
	}
	return out
}
