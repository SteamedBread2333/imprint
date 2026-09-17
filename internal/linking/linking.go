package linking

import (
	"github.com/SteamedBread2333/imprint/internal/shelves"
	"github.com/SteamedBread2333/imprint/internal/shelves/index"
	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

// RuleGet bundles a vault rule with resolved document excerpts for agents.
type RuleGet struct {
	*imprint.Record
	ResolvedSources []imprint.ResolvedSource `json:"resolved_sources,omitempty"`
}

// ChunkGet bundles a document chunk with inbound imprint references.
type ChunkGet struct {
	index.Chunk
	ReferencedRules []imprint.ReferencedRule `json:"referenced_rules,omitempty"`
	CitedRules      []shelves.CitedRule      `json:"cited_rules,omitempty"`
}

// FindResult is the enriched find payload when shelves participates.
type FindResult struct {
	Rules     []imprint.EnrichedFindHit `json:"rules"`
	Documents []index.SearchHit         `json:"documents"`
	Links     []imprint.FindLink        `json:"links,omitempty"`
}

// EnrichRuleGet attaches resolved sources to a vault record.
func EnrichRuleGet(st *index.Store, rec *imprint.Record) RuleGet {
	return RuleGet{
		Record:          rec,
		ResolvedSources: shelves.ResolveSources(st, rec.Sources),
	}
}

// EnrichChunkGet attaches imprints linked via vault sources (and optional markdown cites).
func EnrichChunkGet(v *imprint.Vault, st *index.Store, c index.Chunk) (ChunkGet, error) {
	ref, err := shelves.ReferencedRules(v, c)
	if err != nil {
		return ChunkGet{}, err
	}
	return ChunkGet{
		Chunk:           c,
		ReferencedRules: ref,
		CitedRules:      shelves.CitedRules(st, c.ID),
	}, nil
}

// EnrichFind builds agent-oriented find results with sources, excerpts, and links.
func EnrichFind(v *imprint.Vault, st *index.Store, scope []string, query string, topK int) (*FindResult, []imprint.FindHit, error) {
	hits, err := v.Find(scope, query, topK)
	if err != nil {
		return nil, nil, err
	}
	if st == nil {
		return nil, hits, nil
	}
	ids := make([]string, len(hits))
	for i, h := range hits {
		ids[i] = h.ID
	}
	sourcesByID, err := shelves.SourcesByIDs(v, ids)
	if err != nil {
		return nil, nil, err
	}
	enriched := shelves.EnrichFindHits(st, hits, sourcesByID)
	var docs []index.SearchHit
	if query != "" {
		docs = st.SearchStore(query, topK)
	}
	links := shelves.BuildFindLinks(v, st, enriched, docs)
	return &FindResult{
		Rules:     enriched,
		Documents: docs,
		Links:     links,
	}, hits, nil
}
