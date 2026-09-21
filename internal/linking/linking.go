package linking

import (
	"time"

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

// CompactFindResult is the default token-bounded MCP recall payload.
type CompactFindResult struct {
	Rules     []imprint.CompactFindHit `json:"rules"`
	Documents []index.SearchHit        `json:"documents"`
	Links     []imprint.FindLink       `json:"links,omitempty"`
	Conflicts []imprint.ConflictSet    `json:"conflict_set,omitempty"`
}

// SourcePointer identifies a resolved source without inlining its text.
type SourcePointer struct {
	Path              string `json:"path"`
	Heading           string `json:"heading,omitempty"`
	ChunkID           string `json:"chunk_id,omitempty"`
	StaleChunk        bool   `json:"stale_chunk,omitempty"`
	OutOfIndex        bool   `json:"out_of_index,omitempty"`
	HeadingUnresolved bool   `json:"heading_unresolved,omitempty"`
}

// CompactRuleGet is the default rule get payload. Evidence is opt-in and
// bounded; source text stays out of the response.
type CompactRuleGet struct {
	ID                 string             `json:"id"`
	Claim              string             `json:"claim"`
	Scope              []string           `json:"scope"`
	Confidence         float64            `json:"confidence"`
	Status             imprint.Status     `json:"status"`
	ReinforcementCount int                `json:"reinforcement_count"`
	Supersedes         []string           `json:"supersedes,omitempty"`
	Related            []string           `json:"related,omitempty"`
	ConflictsWith      []string           `json:"conflicts_with,omitempty"`
	Sources            []imprint.DocRef   `json:"sources,omitempty"`
	ResolvedSources    []SourcePointer    `json:"resolved_sources,omitempty"`
	ReferencedBy       []imprint.Backlink `json:"referenced_by,omitempty"`
	EvidenceCount      int                `json:"evidence_count"`
	LastEvidenceAt     *time.Time         `json:"last_evidence_at,omitempty"`
	EvidenceLog        []imprint.Evidence `json:"evidence_log,omitempty"`
}

// EnrichRuleGet attaches resolved sources to a vault record.
func EnrichRuleGet(st *index.Store, rec *imprint.Record) RuleGet {
	return RuleGet{
		Record:          rec,
		ResolvedSources: shelves.ResolveSources(st, rec.Sources),
	}
}

// CompactRule projects a full record into the default agent-facing shape.
// evidenceLimit <= 0 keeps evidence folded.
func CompactRule(rec *imprint.Record, resolved []imprint.ResolvedSource, evidenceLimit int) CompactRuleGet {
	out := CompactRuleGet{
		ID:                 rec.ID,
		Claim:              rec.Claim,
		Scope:              rec.Scope,
		Confidence:         rec.Confidence,
		Status:             rec.Status,
		ReinforcementCount: rec.ReinforcementCount,
		Supersedes:         rec.Supersedes,
		Related:            rec.Related,
		ConflictsWith:      rec.ConflictsWith,
		Sources:            rec.Sources,
		ReferencedBy:       rec.ReferencedBy,
		EvidenceCount:      len(rec.EvidenceLog),
	}
	for _, src := range resolved {
		out.ResolvedSources = append(out.ResolvedSources, SourcePointer{
			Path: src.Path, Heading: src.Heading, ChunkID: src.ChunkID,
			StaleChunk: src.StaleChunk, OutOfIndex: src.OutOfIndex,
			HeadingUnresolved: src.HeadingUnresolved,
		})
	}
	if len(rec.EvidenceLog) > 0 {
		last := rec.EvidenceLog[len(rec.EvidenceLog)-1].At
		out.LastEvidenceAt = &last
	}
	if evidenceLimit > 0 {
		for i := len(rec.EvidenceLog) - 1; i >= 0 && len(out.EvidenceLog) < evidenceLimit; i-- {
			out.EvidenceLog = append(out.EvidenceLog, rec.EvidenceLog[i])
		}
	}
	return out
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
func EnrichFind(v *imprint.Vault, st *index.Store, scope []string, query, queryLocal string, topK int) (*FindResult, []imprint.FindHit, error) {
	hits, err := v.FindMerged(scope, query, queryLocal, topK)
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
	localQ := imprint.EffectiveQueryLocal(query, queryLocal)
	docs := index.SearchStoreMerged(st, query, localQ, topK)
	docs = shelves.FilterFindDocuments(docs, shelves.FindDocMinRelativeScore)
	links := shelves.BuildFindLinks(v, st, enriched, docs)
	return &FindResult{
		Rules:     enriched,
		Documents: docs,
		Links:     links,
	}, hits, nil
}

// CompactFind projects enriched find results and exposes explicit conflicts.
func CompactFind(v *imprint.Vault, full *FindResult) (*CompactFindResult, error) {
	out := &CompactFindResult{
		Rules:     make([]imprint.CompactFindHit, 0, len(full.Rules)),
		Documents: full.Documents,
		Links:     full.Links,
	}
	ids := make([]string, 0, len(full.Rules))
	for _, hit := range full.Rules {
		out.Rules = append(out.Rules, hit.FindHit.Compact())
		ids = append(ids, hit.ID)
	}
	conflicts, err := v.ConflictsAmong(ids)
	if err != nil {
		return nil, err
	}
	out.Conflicts = conflicts
	return out, nil
}
