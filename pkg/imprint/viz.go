package imprint

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//go:embed dashboard.html
var dashboardHTML []byte

//go:embed cytoscape.min.js
var cytoscapeJS []byte

type vizNode struct {
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

type vizEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Kind   string `json:"kind"`
}

type vizData struct {
	GeneratedAt string    `json:"generated_at"`
	Nodes       []vizNode `json:"nodes"`
	Edges       []vizEdge `json:"edges"`
}

func (v *Vault) graph(includeArchived bool) (vizData, []*Record, error) {
	recs, err := v.loadAll()
	if err != nil {
		return vizData{}, nil, err
	}
	data := vizData{
		GeneratedAt: v.instant().Format(time.RFC3339),
		Nodes:       []vizNode{},
		Edges:       []vizEdge{},
	}
	ids := map[string]struct{}{}
	var kept []*Record
	for _, r := range recs {
		if !includeArchived && r.Status != StatusActive {
			continue
		}
		ids[r.ID] = struct{}{}
		kept = append(kept, r)
		data.Nodes = append(data.Nodes, vizNode{
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
		data.Edges = append(data.Edges, vizEdge{Source: src, Target: dst, Kind: kind})
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

func mermaidGraph(data vizData) string {
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

func renderHTML(data vizData) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(true)
	if err := enc.Encode(data); err != nil {
		return nil, err
	}
	payload := bytes.TrimSpace(buf.Bytes())
	if !bytes.Contains(dashboardHTML, []byte("__IMPRINT_DATA__")) {
		return nil, fmt.Errorf("dashboard template missing data marker")
	}
	if !bytes.Contains(dashboardHTML, []byte("__CYTOSCAPE_JS__")) {
		return nil, fmt.Errorf("dashboard template missing cytoscape marker")
	}
	html := bytes.Replace(dashboardHTML, []byte("__IMPRINT_DATA__"), payload, 1)
	return bytes.Replace(html, []byte("__CYTOSCAPE_JS__"), cytoscapeJS, 1), nil
}

// Viz writes an HTML dashboard or a mermaid graph.
func (v *Vault) Viz(out, format string, includeArchived bool) (*VizResult, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = "html"
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
	case "html":
		if out == "" {
			out = filepath.Join(v.Dir, "dashboard.html")
		}
		html, err := renderHTML(data)
		if err != nil {
			return nil, err
		}
		if err := writeFileAtomic(out, html); err != nil {
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
	default:
		return nil, fmt.Errorf("unknown viz format %q (html|mermaid|notes)", format)
	}
}
