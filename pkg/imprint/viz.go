package imprint

import "time"

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
	QueryLocal         string     `json:"query_local,omitempty"`
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

// Graph returns vault graph data for HTTP APIs and desk.
func (v *Vault) Graph(includeArchived bool) (GraphData, error) {
	data, _, err := v.graph(includeArchived)
	return data, err
}

func graphNode(recs []*Record, r *Record) GraphNode {
	return GraphNode{
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
		QueryLocal:         r.QueryLocal,
		LastTouchedAt:      r.LastTouchedAt,
		ReferencedBy:       backlinksFrom(recs, r.ID),
	}
}

func backlinksFrom(recs []*Record, id string) []Backlink {
	var out []Backlink
	for _, r := range recs {
		if r.ID == id {
			continue
		}
		for _, x := range r.Supersedes {
			if x == id {
				out = append(out, Backlink{ID: r.ID, Kind: "supersedes"})
			}
		}
		for _, x := range r.Related {
			if x == id {
				out = append(out, Backlink{ID: r.ID, Kind: "related"})
			}
		}
		for _, x := range r.ConflictsWith {
			if x == id {
				out = append(out, Backlink{ID: r.ID, Kind: "conflicts_with"})
			}
		}
	}
	return out
}

func (v *Vault) graph(includeArchived bool) (GraphData, []*Record, error) {
	recs, err := v.loadAll()
	if err != nil {
		return GraphData{}, nil, err
	}
	byID := make(map[string]*Record, len(recs))
	for _, r := range recs {
		byID[r.ID] = r
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
		data.Nodes = append(data.Nodes, graphNode(recs, r))
	}
	if !includeArchived {
		linked := map[string]struct{}{}
		for _, r := range kept {
			for _, id := range r.Supersedes {
				linked[id] = struct{}{}
			}
			for _, id := range r.Related {
				linked[id] = struct{}{}
			}
			for _, id := range r.ConflictsWith {
				linked[id] = struct{}{}
			}
		}
		for id := range linked {
			if _, ok := ids[id]; ok {
				continue
			}
			r, ok := byID[id]
			if !ok {
				continue
			}
			ids[id] = struct{}{}
			data.Nodes = append(data.Nodes, graphNode(recs, r))
		}
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
