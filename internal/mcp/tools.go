package mcp

import (
	"context"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/SteamedBread2333/imprint/internal/linking"
	"github.com/SteamedBread2333/imprint/internal/shelves"
	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

func registerTools(server *sdkmcp.Server, v *imprint.Vault, shelvesSvc *shelves.Service) {
	s := &vaultTools{v: v, shelves: shelvesSvc}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "find",
		Description: "Search active rules (scope AND-filtered; optional query + query_local dual BM25, merge by id max score). Call at most ONCE per user message — prepare scope, query, and query_local together before invoking; reuse results for write classification. query_local is LLM-distilled local-language terms, not user verbatim. Reference/核对 only; do not block code reads. Shelves+MCP returns { rules, documents, links }. Weak document hits and co_search links below 75% of top doc score are dropped. Prefer rule claim over documents when answering. links: sources (persistent), cited_by, co_search (assist only). If a resolved_source is heading_unresolved, do not treat another section excerpt as the basis.",
	}, s.find)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "add",
		Description: "Add after ADD classification only (no matching active rule; not a one-off IGNORE). Required: English imperative claim, scope, text (user verbatim). Optional query_local (LLM local-language search terms, not verbatim; vault merges CJK evidence tokens). Optional sources [{path, heading?, chunk?}] only when a document excerpt substantively states the policy—not topical overlap; vault only, no markdown edits. Omit confidence (0.6 default); 0.85 when the user corrects; never 0.9 on add—tier 0.9 requires reinforce.",
	}, s.add)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "reinforce",
		Description: "User confirmed the same existing preference. +0.1 confidence, cap 0.95. Optional evidence note. Optional query_local update (LLM local terms, not verbatim).",
	}, s.reinforce)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "supersede",
		Description: "Preference changed or old claim no longer holds: archive old_id, write new active English claim+scope. Inherits old sources and query_local unless overridden. Optional text = user verbatim. Optional query_local (omit to inherit). Optional sources [{path, heading?, chunk?}] replace inherited rule→doc links in vault.",
	}, s.supersede)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "forget",
		Description: "User negated in plain speech (don't record / forget that). Delete the rule and strip inbound links. Find first, then forget — do not ask the user for a rule id.",
	}, s.forget)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get",
		Description: "Load by id. r-… → vault + referenced_by + resolved_sources (MCP+shelves). If heading_unresolved, keep the declared heading and do not use another section's excerpt. chunk id → document + referenced_rules (vault sources reverse; no markdown edits) + cited_rules ([[r-…]] / [imprint:r-…] only).",
	}, s.get)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "list",
		Description: "Filter rules by status, scope (AND), query, min_confidence, since, limit. Not a substitute for the single find before write classification.",
	}, s.list)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "show",
		Description: "Census of active rules when the user asks what is recorded. Optional limit.",
	}, s.show)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "sweep",
		Description: "Maintainer decay: subtract confidence for untouched rules and mark dormant. Not a per-turn agent default.",
	}, s.sweep)
}

type vaultTools struct {
	v       *imprint.Vault
	shelves *shelves.Service
}

type findArgs struct {
	Scope      string `json:"scope" jsonschema:"comma-separated scope tags (AND); prepare with query and query_local before the single find"`
	Query      string `json:"query,omitempty" jsonschema:"BM25 for vault and shelves (policy/English terms); pass together with query_local when local terms differ"`
	QueryLocal string `json:"query_local,omitempty" jsonschema:"LLM-distilled local-language search terms, not user verbatim; extra BM25 pass, merge by id max score"`
	TopK       int    `json:"top_k,omitempty" jsonschema:"max hits (default 5)"`
}

func (s *vaultTools) find(_ context.Context, _ *sdkmcp.CallToolRequest, args findArgs) (*sdkmcp.CallToolResult, any, error) {
	topK := args.TopK
	if topK == 0 {
		topK = imprint.DefaultTopK
	}
	scope := splitCSV(args.Scope)
	query := args.Query
	queryLocal := args.QueryLocal

	if s.shelves != nil && shelves.MatchFindQuery(s.shelves.Config(), query, queryLocal) {
		enriched, _, err := linking.EnrichFind(s.v, s.shelves.IndexStore(), scope, query, queryLocal, topK)
		if err != nil {
			return toolErr(err), nil, nil
		}
		return jsonOK(enriched)
	}

	hits, err := s.v.FindMerged(scope, query, queryLocal, topK)
	if err != nil {
		return toolErr(err), nil, nil
	}
	if s.shelves != nil && s.shelves.IndexStore() != nil && len(hits) > 0 {
		ids := make([]string, len(hits))
		for i, h := range hits {
			ids[i] = h.ID
		}
		sourcesByID, err := shelves.SourcesByIDs(s.v, ids)
		if err != nil {
			return toolErr(err), nil, nil
		}
		return jsonOK(shelves.EnrichFindHits(s.shelves.IndexStore(), hits, sourcesByID))
	}
	return jsonOK(hits)
}

type docRefArg struct {
	Path    string `json:"path,omitempty" jsonschema:"workspace-relative document path"`
	Heading string `json:"heading,omitempty" jsonschema:"optional section title; matched after markdown inline to plain text"`
	Chunk   string `json:"chunk,omitempty" jsonschema:"optional shelves chunk id"`
}

func parseDocRefs(in []docRefArg) []imprint.DocRef {
	if len(in) == 0 {
		return nil
	}
	out := make([]imprint.DocRef, 0, len(in))
	for _, d := range in {
		out = append(out, imprint.DocRef{Path: d.Path, Heading: d.Heading, Chunk: d.Chunk})
	}
	return out
}

type addArgs struct {
	Claim      string      `json:"claim" jsonschema:"English imperative claim for agents; user wording in text, local search terms in query_local"`
	Scope      string      `json:"scope" jsonschema:"comma-separated scope tags"`
	Text       string      `json:"text" jsonschema:"user's original words (evidence); do not copy into query_local"`
	Confidence float64     `json:"confidence,omitempty" jsonschema:"omit for 0.6; 0.85 only when user corrects; never 0.9 on add"`
	QueryLocal string      `json:"query_local,omitempty" jsonschema:"LLM local-language search terms, not verbatim; vault merges CJK evidence tokens"`
	Sources    []docRefArg `json:"sources,omitempty" jsonschema:"[{path, heading?, chunk?}] only when excerpt substantively supports the claim, not topical overlap; vault only"`
}

func (s *vaultTools) add(_ context.Context, _ *sdkmcp.CallToolRequest, args addArgs) (*sdkmcp.CallToolResult, any, error) {
	res, err := s.v.AddRecord(args.Claim, splitCSV(args.Scope), args.Text, args.Confidence, nil, nil, nil, parseDocRefs(args.Sources), args.QueryLocal)
	if err != nil {
		return toolErr(err), nil, nil
	}
	return jsonOK(res)
}

type reinforceArgs struct {
	ID         string `json:"id" jsonschema:"rule id from find (do not ask the user)"`
	Evidence   string `json:"evidence,omitempty" jsonschema:"optional evidence note"`
	QueryLocal string `json:"query_local,omitempty" jsonschema:"optional LLM local-language terms, not verbatim; updates stored query_local"`
}

func (s *vaultTools) reinforce(_ context.Context, _ *sdkmcp.CallToolRequest, args reinforceArgs) (*sdkmcp.CallToolResult, any, error) {
	res, err := s.v.ReinforceQueryLocal(args.ID, args.Evidence, args.QueryLocal)
	if err != nil {
		return toolErr(err), nil, nil
	}
	return jsonOK(res)
}

type supersedeArgs struct {
	OldID      string      `json:"old_id" jsonschema:"rule id to archive (from find; do not ask the user)"`
	Claim      string      `json:"claim" jsonschema:"English imperative new claim"`
	Scope      string      `json:"scope" jsonschema:"comma-separated scope tags"`
	Reason     string      `json:"reason,omitempty" jsonschema:"optional supersede reason (evidence if text omitted)"`
	Text       string      `json:"text,omitempty" jsonschema:"user's original words for the new rule"`
	QueryLocal string      `json:"query_local,omitempty" jsonschema:"LLM local-language terms, not verbatim; omit to inherit from old rule"`
	Sources    []docRefArg `json:"sources,omitempty" jsonschema:"[{path, heading?, chunk?}]; omit to inherit from old rule"`
}

func (s *vaultTools) supersede(_ context.Context, _ *sdkmcp.CallToolRequest, args supersedeArgs) (*sdkmcp.CallToolResult, any, error) {
	res, err := s.v.SupersedeWithSources(args.OldID, args.Claim, splitCSV(args.Scope), args.Reason, args.Text, parseDocRefs(args.Sources), args.QueryLocal)
	if err != nil {
		return toolErr(err), nil, nil
	}
	return jsonOK(res)
}

type forgetArgs struct {
	ID string `json:"id" jsonschema:"rule id from find (do not ask the user)"`
}

func (s *vaultTools) forget(_ context.Context, _ *sdkmcp.CallToolRequest, args forgetArgs) (*sdkmcp.CallToolResult, any, error) {
	res, err := s.v.Forget(args.ID)
	if err != nil {
		return toolErr(err), nil, nil
	}
	return jsonOK(res)
}

type getArgs struct {
	ID string `json:"id" jsonschema:"r-… rule id, or shelves chunk id"`
}

func (s *vaultTools) get(_ context.Context, _ *sdkmcp.CallToolRequest, args getArgs) (*sdkmcp.CallToolResult, any, error) {
	if s.shelves != nil && s.shelves.HasChunk(args.ID) {
		if c, ok := s.shelves.ChunkByID(args.ID); ok {
			out, err := linking.EnrichChunkGet(s.v, s.shelves.IndexStore(), c)
			if err != nil {
				return toolErr(err), nil, nil
			}
			return jsonOK(out)
		}
	}
	rec, err := s.v.Get(args.ID)
	if err != nil {
		return toolErr(err), nil, nil
	}
	if s.shelves != nil {
		return jsonOK(linking.EnrichRuleGet(s.shelves.IndexStore(), rec))
	}
	return jsonOK(rec)
}

type listArgs struct {
	Status        string  `json:"status,omitempty" jsonschema:"active, dormant, or superseded"`
	Scope         string  `json:"scope,omitempty" jsonschema:"comma-separated scope tags (AND filter)"`
	Query         string  `json:"query,omitempty" jsonschema:"optional BM25 filter on rules"`
	MinConfidence float64 `json:"min_confidence,omitempty" jsonschema:"minimum confidence threshold"`
	Since         string  `json:"since,omitempty" jsonschema:"YYYY-MM-DD or RFC3339; rules touched since"`
	Limit         int     `json:"limit,omitempty" jsonschema:"max rows (default unlimited)"`
}

func (s *vaultTools) list(_ context.Context, _ *sdkmcp.CallToolRequest, args listArgs) (*sdkmcp.CallToolResult, any, error) {
	when, err := parseSince(args.Since)
	if err != nil {
		return toolErr(err), nil, nil
	}
	items, err := s.v.ListFilter(imprint.ListFilter{
		Status:        args.Status,
		Scope:         splitCSV(args.Scope),
		MinConfidence: args.MinConfidence,
		Query:         args.Query,
		Since:         when,
		Limit:         args.Limit,
	})
	if err != nil {
		return toolErr(err), nil, nil
	}
	return jsonOK(items)
}

type showArgs struct {
	Limit int `json:"limit,omitempty" jsonschema:"max active rules to return (default all)"`
}

func (s *vaultTools) show(_ context.Context, _ *sdkmcp.CallToolRequest, args showArgs) (*sdkmcp.CallToolResult, any, error) {
	recs, err := s.v.Show(args.Limit)
	if err != nil {
		return toolErr(err), nil, nil
	}
	return jsonOK(recs)
}

type sweepArgs struct {
	DecayDays        int     `json:"decay_days,omitempty" jsonschema:"days without touch before decay (default 90)"`
	DecayAmount      float64 `json:"decay_amount,omitempty" jsonschema:"confidence subtracted per decay (default 0.05)"`
	DormantThreshold float64 `json:"dormant_threshold,omitempty" jsonschema:"mark dormant below this confidence (default 0.3)"`
}

func (s *vaultTools) sweep(_ context.Context, _ *sdkmcp.CallToolRequest, args sweepArgs) (*sdkmcp.CallToolResult, any, error) {
	res, err := s.v.Sweep(args.DecayDays, args.DecayAmount, args.DormantThreshold)
	if err != nil {
		return toolErr(err), nil, nil
	}
	return jsonOK(res)
}
