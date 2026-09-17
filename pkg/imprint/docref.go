package imprint

import "strings"

// DocRef points a rule at a workspace document (path, optional heading, optional chunk id).
type DocRef struct {
	Path    string `yaml:"path,omitempty" json:"path,omitempty"`
	Heading string `yaml:"heading,omitempty" json:"heading,omitempty"`
	Chunk   string `yaml:"chunk,omitempty" json:"chunk,omitempty"`
}

// ResolvedSource is a DocRef resolved against the shelves index for agent recall.
type ResolvedSource struct {
	Path       string `json:"path"`
	Heading    string `json:"heading,omitempty"`
	ChunkID    string `json:"chunk_id,omitempty"`
	Snippet    string `json:"snippet,omitempty"`
	LineStart  int    `json:"line_start,omitempty"`
	LineEnd    int    `json:"line_end,omitempty"`
	StaleChunk bool   `json:"stale_chunk,omitempty"`
	OutOfIndex bool   `json:"out_of_index,omitempty"`
}

// FindLink connects a rule and a document chunk in find results.
type FindLink struct {
	RuleID  string  `json:"rule_id"`
	ChunkID string  `json:"chunk_id"`
	Kind    string  `json:"kind"`
	Score   float64 `json:"score"`
}

// EnrichedFindHit is one rule hit with optional source excerpts for agents.
type EnrichedFindHit struct {
	FindHit
	Sources         []DocRef         `json:"sources,omitempty"`
	ResolvedSources []ResolvedSource `json:"resolved_sources,omitempty"`
}

// ReferencedRule is an imprint whose vault sources point at a document path or chunk.
type ReferencedRule struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Claim string `json:"claim,omitempty"`
}

// RulesReferencingDoc returns imprints whose sources point at path/heading/chunkID.
func (v *Vault) RulesReferencingDoc(path, heading, chunkID string) ([]ReferencedRule, error) {
	path = filepathSlash(strings.TrimSpace(strings.TrimPrefix(path, "./")))
	heading = strings.TrimSpace(heading)
	chunkID = strings.TrimSpace(chunkID)
	if path == "" && chunkID == "" {
		return nil, nil
	}
	recs, err := v.loadAll()
	if err != nil {
		return nil, err
	}
	var out []ReferencedRule
	seen := map[string]struct{}{}
	for _, r := range recs {
		if r.Status != StatusActive {
			continue
		}
		for _, src := range r.Sources {
			if !sourceMatchesDoc(src, path, heading, chunkID) {
				continue
			}
			if _, ok := seen[r.ID]; ok {
				break
			}
			seen[r.ID] = struct{}{}
			out = append(out, ReferencedRule{
				ID:    r.ID,
				Kind:  "sources",
				Claim: r.Claim,
			})
			break
		}
	}
	return out, nil
}

func sourceMatchesDoc(src DocRef, path, heading, chunkID string) bool {
	srcPath := filepathSlash(strings.TrimSpace(strings.TrimPrefix(src.Path, "./")))
	srcHeading := strings.TrimSpace(src.Heading)
	srcChunk := strings.TrimSpace(src.Chunk)
	if srcChunk != "" && chunkID != "" && srcChunk == chunkID {
		return true
	}
	if srcPath == "" || path == "" || srcPath != path {
		return false
	}
	if srcHeading == "" {
		return true
	}
	if heading == "" {
		return false
	}
	return strings.EqualFold(srcHeading, heading) || strings.Contains(strings.ToLower(heading), strings.ToLower(srcHeading))
}

func cleanDocRefs(refs []DocRef) []DocRef {
	if len(refs) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	var out []DocRef
	for _, r := range refs {
		r.Path = strings.TrimSpace(r.Path)
		r.Path = strings.TrimPrefix(r.Path, "./")
		r.Path = filepathSlash(r.Path)
		r.Heading = strings.TrimSpace(r.Heading)
		r.Chunk = strings.TrimSpace(r.Chunk)
		if r.Path == "" && r.Chunk == "" {
			continue
		}
		key := r.Path + "\x00" + r.Heading + "\x00" + r.Chunk
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, r)
	}
	return out
}

func filepathSlash(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}
