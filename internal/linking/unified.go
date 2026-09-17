package linking

import (
	"time"

	"github.com/SteamedBread2333/imprint/internal/shelves"
	"github.com/SteamedBread2333/imprint/internal/shelves/index"
	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

// UnifiedNode is one node in GET /graph/unified.
type UnifiedNode struct {
	ID                 string             `json:"id"`
	Kind               string             `json:"kind"`
	Label              string             `json:"label,omitempty"`
	Path               string             `json:"path,omitempty"`
	Heading            string             `json:"heading,omitempty"`
	Claim              string             `json:"claim,omitempty"`
	Scope              []string           `json:"scope,omitempty"`
	Confidence         float64            `json:"confidence,omitempty"`
	Status             string             `json:"status,omitempty"`
	ReinforcementCount int                `json:"reinforcement_count,omitempty"`
	Supersedes         []string           `json:"supersedes,omitempty"`
	Related            []string           `json:"related,omitempty"`
	ConflictsWith      []string           `json:"conflicts_with,omitempty"`
	EvidenceLog        []imprint.Evidence `json:"evidence_log,omitempty"`
	LastTouchedAt      time.Time          `json:"last_touched_at,omitempty"`
	ReferencedBy       []imprint.Backlink `json:"referenced_by,omitempty"`
}

// UnifiedEdge links nodes in GET /graph/unified.
type UnifiedEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Kind   string `json:"kind"`
}

// UnifiedGraph merges vault rules, shelves documents, and cross links.
type UnifiedGraph struct {
	GeneratedAt string        `json:"generated_at"`
	Nodes       []UnifiedNode `json:"nodes"`
	Edges       []UnifiedEdge `json:"edges"`
}

// BuildUnifiedGraph composes rule graph, document graph, and sources / cited_by edges.
func BuildUnifiedGraph(v *imprint.Vault, st *index.Store, includeArchived bool) (UnifiedGraph, error) {
	out := UnifiedGraph{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Nodes:       []UnifiedNode{},
		Edges:       []UnifiedEdge{},
	}
	if v == nil {
		return out, nil
	}
	ruleGraph, err := v.Graph(includeArchived)
	if err != nil {
		return UnifiedGraph{}, err
	}
	seenNode := map[string]struct{}{}
	seenEdge := map[string]struct{}{}
	addNode := func(n UnifiedNode) {
		if n.ID == "" {
			return
		}
		if _, ok := seenNode[n.ID]; ok {
			return
		}
		seenNode[n.ID] = struct{}{}
		out.Nodes = append(out.Nodes, n)
	}
	addEdge := func(source, target, kind string) {
		if source == "" || target == "" || source == target {
			return
		}
		key := source + "\x00" + target + "\x00" + kind
		if _, ok := seenEdge[key]; ok {
			return
		}
		if _, ok := seenNode[source]; !ok {
			return
		}
		if _, ok := seenNode[target]; !ok {
			return
		}
		seenEdge[key] = struct{}{}
		out.Edges = append(out.Edges, UnifiedEdge{Source: source, Target: target, Kind: kind})
	}

	for _, n := range ruleGraph.Nodes {
		label := n.Claim
		if len(label) > 48 {
			label = label[:46] + "…"
		}
		addNode(UnifiedNode{
			ID:                 n.ID,
			Kind:               "rule",
			Label:              label,
			Claim:              n.Claim,
			Scope:              n.Scope,
			Confidence:         n.Confidence,
			Status:             n.Status,
			ReinforcementCount: n.ReinforcementCount,
			Supersedes:         n.Supersedes,
			Related:            n.Related,
			ConflictsWith:      n.ConflictsWith,
			EvidenceLog:        n.EvidenceLog,
			LastTouchedAt:      n.LastTouchedAt,
			ReferencedBy:       n.ReferencedBy,
		})
	}
	for _, e := range ruleGraph.Edges {
		addEdge(e.Source, e.Target, e.Kind)
	}

	var chunks []index.Chunk
	if st != nil {
		chunks = st.Chunks
	}
	docGraph := shelves.BuildGraph(chunks)
	for _, n := range docGraph.Nodes {
		label := n.Heading
		if label == "" {
			label = n.Path
		}
		addNode(UnifiedNode{
			ID:      n.ID,
			Kind:    n.Kind,
			Label:   label,
			Path:    n.Path,
			Heading: n.Heading,
		})
	}
	for _, e := range docGraph.Edges {
		addEdge(e.Source, e.Target, e.Kind)
	}

	for _, n := range ruleGraph.Nodes {
		rec, err := v.Get(n.ID)
		if err != nil {
			continue
		}
		for _, ref := range rec.Sources {
			rs := shelves.ResolveSources(st, []imprint.DocRef{ref})
			if len(rs) == 0 {
				continue
			}
			res := rs[0]
			if res.ChunkID != "" {
				addEdge(rec.ID, res.ChunkID, "sources")
				continue
			}
			if res.Path != "" {
				fileID := "file:" + res.Path
				addEdge(rec.ID, fileID, "sources")
			}
		}
	}
	if st != nil {
		for _, ref := range st.RuleRefs {
			addEdge(ref.ChunkID, ref.RuleID, "cited_by")
		}
	}
	return out, nil
}
