package imprint

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GraphNode is one rule in the vault graph JSON.
type GraphNode struct {
	ID                 string     `json:"id"`
	Claim              string     `json:"claim"`
	Scope              []string   `json:"scope"`
	Confidence         float64    `json:"confidence"`
	Status             string     `json:"status"`
	ReinforcementCount int        `json:"reinforcement_count"`
	Supersedes         []string   `json:"supersedes"`
	Related            []string   `json:"related"`
	ConflictsWith      []string   `json:"conflicts_with"`
	EvidenceLog        []Evidence `json:"evidence_log"`
	LastTouchedAt      time.Time  `json:"last_touched_at"`
	ReferencedBy       []Backlink `json:"referenced_by,omitempty"`
}

// GraphEdge links two rules in the vault graph.
type GraphEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Kind   string `json:"kind"`
}

// GraphData is the JSON shape for GET /graph and desk UI.
type GraphData struct {
	GeneratedAt string      `json:"generated_at"`
	Nodes       []GraphNode `json:"nodes"`
	Edges       []GraphEdge `json:"edges"`
}

// Graph returns vault graph data for HTTP APIs and plugins.
func (v *Vault) Graph(includeArchived bool) (GraphData, error) {
	data, _, err := v.graph(includeArchived)
	return data, err
}

func (v *Vault) graph(includeArchived bool) (GraphData, []*Record, error) {
	recs, err := v.loadAll()
	if err != nil {
		return GraphData{}, nil, err
	}
	data := GraphData{
		GeneratedAt: v.instant().Format(time.RFC3339),
		Nodes:       []GraphNode{},
		Edges:       []GraphEdge{},
	}
	ids := map[string]struct{}{}
	var kept []*Record
	for _, r := range recs {
		if !includeArchived && r.Status != StatusActive {
			continue
		}
		ids[r.ID] = struct{}{}
		kept = append(kept, r)
		data.Nodes = append(data.Nodes, GraphNode{
			ID:                 r.ID,
			Claim:              r.Claim,
			Scope:              r.Scope,
			Confidence:         r.Confidence,
			Status:             string(r.Status),
			ReinforcementCount: r.ReinforcementCount,
			Supersedes:         r.Supersedes,
			Related:            r.Related,
			ConflictsWith:      r.ConflictsWith,
			EvidenceLog:        r.EvidenceLog,
			LastTouchedAt:      r.LastTouchedAt,
			ReferencedBy:       backlinksFrom(recs, r.ID),
		})
	}
	addEdge := func(src, dst, kind string) {
		if src == "" || dst == "" || src == dst {
			return
		}
		if _, ok := ids[src]; !ok {
			return
		}
		if _, ok := ids[dst]; !ok {
			return
		}
		data.Edges = append(data.Edges, GraphEdge{Source: src, Target: dst, Kind: kind})
	}
	for _, r := range kept {
		for _, old := range r.Supersedes {
			addEdge(r.ID, old, "supersedes")
		}
		for _, rel := range r.Related {
			if containsID(r.Supersedes, rel) {
				continue
			}
			addEdge(r.ID, rel, "related")
		}
		for _, c := range r.ConflictsWith {
			addEdge(r.ID, c, "conflicts_with")
		}
	}
	return data, kept, nil
}

func containsID(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

func mermaidGraph(data GraphData) string {
	var b strings.Builder
	b.WriteString("graph LR\n")
	ident := func(id string) string {
		return strings.ReplaceAll(id, "-", "_")
	}
	esc := func(s string) string {
		s = strings.ReplaceAll(s, `"`, `'`)
		if len(s) > 42 {
			s = s[:42] + "…"
		}
		return s
	}
	if len(data.Nodes) == 0 {
		b.WriteString("  empty[no imprints]\n")
		return b.String()
	}
	for _, n := range data.Nodes {
		b.WriteString(fmt.Sprintf("  %s[\"%s<br/>%s\"]\n", ident(n.ID), n.ID, esc(n.Claim)))
	}
	for _, e := range data.Edges {
		arrow := "---"
		switch e.Kind {
		case "supersedes":
			arrow = "-- supersedes -->"
		case "related":
			arrow = "-. related .-"
		case "conflicts_with":
			arrow = "== conflicts =="
		}
		b.WriteString(fmt.Sprintf("  %s %s %s\n", ident(e.Source), arrow, ident(e.Target)))
	}
	return b.String()
}

// Viz writes a mermaid graph or read-only notes/ cards.
func (v *Vault) Viz(out, format string, includeArchived bool) (*VizResult, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = "mermaid"
	}
	if format == "html" {
		return nil, fmt.Errorf("html dashboard removed — use imprint desk open (desk plugin) or imprint viz --format mermaid|notes")
	}
	data, recs, err := v.graph(includeArchived)
	if err != nil {
		return nil, err
	}
	result := &VizResult{RulesCount: len(recs)}
	switch format {
	case "mermaid":
		m := mermaidGraph(data)
		result.Mermaid = m
		if out == "" {
			result.SizeBytes = int64(len(m))
			return result, nil
		}
		if err := writeFileAtomic(out, []byte(m)); err != nil {
			return nil, err
		}
		abs, err := filepath.Abs(out)
		if err != nil {
			return nil, err
		}
		st, err := os.Stat(abs)
		if err != nil {
			return nil, err
		}
		result.Path = abs
		result.SizeBytes = st.Size()
		return result, nil
	case "notes":
		if out == "" {
			out = filepath.Join(v.Dir, "notes")
		}
		n, err := v.writeNotes(out, recs)
		if err != nil {
			return nil, err
		}
		abs, err := filepath.Abs(out)
		if err != nil {
			return nil, err
		}
		result.Path = abs
		result.RulesCount = n
		st, err := os.Stat(abs)
		if err != nil {
			return nil, err
		}
		result.SizeBytes = st.Size()
		return result, nil
	default:
		return nil, fmt.Errorf("unknown viz format %q (mermaid|notes)", format)
	}
}
