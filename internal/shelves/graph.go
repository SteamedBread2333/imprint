package shelves

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/SteamedBread2333/imprint/internal/shelves/index"
)

// DocGraphNode is one node in the document relationship graph.
type DocGraphNode struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	Heading string `json:"heading,omitempty"`
	Kind    string `json:"kind"`
}

// DocGraphEdge links document nodes.
type DocGraphEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Kind   string `json:"kind"`
}

// DocGraph is returned by GET /docs/graph.
type DocGraph struct {
	GeneratedAt string         `json:"generated_at"`
	Nodes       []DocGraphNode `json:"nodes"`
	Edges       []DocGraphEdge `json:"edges"`
}

// BuildGraph constructs a read-only graph from indexed chunks.
func BuildGraph(chunks []index.Chunk) DocGraph {
	g := DocGraph{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Nodes:       []DocGraphNode{},
		Edges:       []DocGraphEdge{},
	}
	if len(chunks) == 0 {
		return g
	}

	fileNodes := map[string]string{}
	dirNodes := map[string]string{}
	seenNode := map[string]struct{}{}

	addNode := func(id, path, heading, kind string) {
		if _, ok := seenNode[id]; ok {
			return
		}
		seenNode[id] = struct{}{}
		g.Nodes = append(g.Nodes, DocGraphNode{
			ID:      id,
			Path:    path,
			Heading: heading,
			Kind:    kind,
		})
	}
	addEdge := func(source, target, kind string) {
		if source == "" || target == "" || source == target {
			return
		}
		g.Edges = append(g.Edges, DocGraphEdge{Source: source, Target: target, Kind: kind})
	}

	for _, c := range chunks {
		fileID, ok := fileNodes[c.Path]
		if !ok {
			fileID = "file:" + c.Path
			fileNodes[c.Path] = fileID
			addNode(fileID, c.Path, filepath.Base(c.Path), "file")
		}
		addNode(c.ID, c.Path, c.Heading, "chunk")
		addEdge(fileID, c.ID, "contains")

		dir := filepath.ToSlash(filepath.Dir(c.Path))
		for dir != "." && dir != "" {
			dirID, ok := dirNodes[dir]
			if !ok {
				dirID = "dir:" + dir
				dirNodes[dir] = dirID
				addNode(dirID, dir, filepath.Base(dir), "directory")
			}
			parent := filepath.ToSlash(filepath.Dir(dir))
			if parent == "." {
				parent = ""
			}
			if parent != "" {
				parentID, ok := dirNodes[parent]
				if !ok {
					parentID = "dir:" + parent
					dirNodes[parent] = parentID
					addNode(parentID, parent, filepath.Base(parent), "directory")
				}
				addEdge(parentID, dirID, "directory")
			}
			addEdge(dirID, fileID, "directory")
			dir = parent
		}
	}

	byPath := map[string][]index.Chunk{}
	for _, c := range chunks {
		byPath[c.Path] = append(byPath[c.Path], c)
	}
	for _, list := range byPath {
		for i := 1; i < len(list); i++ {
			addEdge(list[i-1].ID, list[i].ID, "sequence")
		}
	}

	// Dedupe edges (directory walks may repeat).
	seenEdge := map[string]struct{}{}
	uniq := g.Edges[:0]
	for _, e := range g.Edges {
		key := e.Source + "\x00" + e.Target + "\x00" + e.Kind
		if _, ok := seenEdge[key]; ok {
			continue
		}
		seenEdge[key] = struct{}{}
		uniq = append(uniq, e)
	}
	g.Edges = uniq
	return g
}

// Stats is shelves index metadata for GET /docs/stats.
type Stats struct {
	ChunkCount  int       `json:"chunk_count"`
	FileCount   int       `json:"file_count"`
	BuiltAt     time.Time `json:"built_at"`
	Fingerprint string    `json:"fingerprint"`
	Roots       []string  `json:"roots,omitempty"`
}

// Status is embedded in GET /health.
type Status struct {
	Enabled     bool   `json:"enabled"`
	Indexed     bool   `json:"indexed"`
	ChunkCount  int    `json:"chunk_count"`
	FileCount   int    `json:"file_count"`
	BuiltAt     string `json:"built_at,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
}

func statusFromStore(cfg Config, st *index.Store) Status {
	s := Status{
		Enabled:    cfg.Enabled,
		Indexed:    cfg.Enabled && st != nil && len(st.Chunks) > 0,
		ChunkCount: len(st.Chunks),
	}
	if st == nil {
		return s
	}
	s.FileCount = st.FileCount
	s.Fingerprint = st.Fingerprint
	if !st.BuiltAt.IsZero() {
		s.BuiltAt = st.BuiltAt.UTC().Format(time.RFC3339)
	}
	return s
}

// ChunkID looks like 16 hex chars from chunkID().
func ChunkID(id string) bool {
	if len(id) != 16 {
		return false
	}
	for _, r := range id {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

// HasChunk reports whether id exists in the loaded store.
func HasChunk(st *index.Store, id string) bool {
	if st == nil || !ChunkID(id) {
		return false
	}
	_, ok := st.ChunkByID(id)
	return ok
}

// MatchQuery is true when shelves should enrich find results with documents.
func MatchQuery(cfg Config, query string) bool {
	return cfg.Enabled && strings.TrimSpace(query) != ""
}
