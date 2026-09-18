package imprint

import (
	"strings"

	"github.com/SteamedBread2333/imprint/internal/vault/model"
)

type DocRef = model.DocRef

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

type ReferencedRule = model.ReferencedRule

// RulesReferencingDoc returns imprints whose sources point at path/heading/chunkID.
func (v *Vault) RulesReferencingDoc(path, heading, chunkID string) ([]ReferencedRule, error) {
	return v.store.RulesReferencingDoc(path, heading, chunkID)
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
