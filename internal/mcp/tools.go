package mcp

import (
	"context"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

func registerTools(server *sdkmcp.Server, v *imprint.Vault) {
	s := &vaultTools{v: v}
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "find",
		Description: "Search active rules: scope tags are AND-filtered, then optional BM25 query ranks matches.",
	}, s.find)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "add",
		Description: "Add a rule after classifying ADD. claim, scope, and text are required.",
	}, s.add)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "reinforce",
		Description: "Reinforce an existing rule (+0.1 confidence, cap 0.95).",
	}, s.reinforce)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "supersede",
		Description: "Archive old_id and write a new rule with claim and scope.",
	}, s.supersede)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "forget",
		Description: "Delete a rule and strip inbound links.",
	}, s.forget)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "get",
		Description: "Load one rule with evidence_log and referenced_by.",
	}, s.get)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "list",
		Description: "List rules with optional status, scope, query, min_confidence, since, limit filters.",
	}, s.list)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "show",
		Description: "Census of active rules (optionally limited).",
	}, s.show)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "sweep",
		Description: "Decay confidence and archive dormant rules.",
	}, s.sweep)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "viz",
		Description: "Generate dashboard HTML, mermaid on stdout, or notes/ cards; returns path and stats.",
	}, s.viz)
}

type vaultTools struct {
	v *imprint.Vault
}

type findArgs struct {
	Scope string `json:"scope" jsonschema:"comma-separated scope tags (AND filter)"`
	Query string `json:"query,omitempty" jsonschema:"optional BM25 query"`
	TopK  int    `json:"top_k,omitempty" jsonschema:"max hits (default 5)"`
}

func (s *vaultTools) find(_ context.Context, _ *sdkmcp.CallToolRequest, args findArgs) (*sdkmcp.CallToolResult, any, error) {
	topK := args.TopK
	if topK == 0 {
		topK = imprint.DefaultTopK
	}
	hits, err := s.v.Find(splitCSV(args.Scope), args.Query, topK)
	if err != nil {
		return toolErr(err), nil, nil
	}
	return jsonOK(hits)
}

type addArgs struct {
	Claim      string  `json:"claim" jsonschema:"imperative claim text"`
	Scope      string  `json:"scope" jsonschema:"comma-separated scope tags"`
	Text       string  `json:"text" jsonschema:"user's original words"`
	Confidence float64 `json:"confidence,omitempty" jsonschema:"starting confidence; 0 uses default 0.6"`
}

func (s *vaultTools) add(_ context.Context, _ *sdkmcp.CallToolRequest, args addArgs) (*sdkmcp.CallToolResult, any, error) {
	res, err := s.v.Add(args.Claim, splitCSV(args.Scope), args.Text, args.Confidence)
	if err != nil {
		return toolErr(err), nil, nil
	}
	return jsonOK(res)
}

type reinforceArgs struct {
	ID       string `json:"id" jsonschema:"rule id"`
	Evidence string `json:"evidence,omitempty" jsonschema:"optional evidence note"`
}

func (s *vaultTools) reinforce(_ context.Context, _ *sdkmcp.CallToolRequest, args reinforceArgs) (*sdkmcp.CallToolResult, any, error) {
	res, err := s.v.Reinforce(args.ID, args.Evidence)
	if err != nil {
		return toolErr(err), nil, nil
	}
	return jsonOK(res)
}

type supersedeArgs struct {
	OldID  string `json:"old_id" jsonschema:"rule id to archive"`
	Claim  string `json:"claim" jsonschema:"new claim"`
	Scope  string `json:"scope" jsonschema:"comma-separated scope tags"`
	Reason string `json:"reason,omitempty"`
	Text   string `json:"text,omitempty" jsonschema:"user's original words for the new rule"`
}

func (s *vaultTools) supersede(_ context.Context, _ *sdkmcp.CallToolRequest, args supersedeArgs) (*sdkmcp.CallToolResult, any, error) {
	res, err := s.v.Supersede(args.OldID, args.Claim, splitCSV(args.Scope), args.Reason, args.Text)
	if err != nil {
		return toolErr(err), nil, nil
	}
	return jsonOK(res)
}

type forgetArgs struct {
	ID string `json:"id" jsonschema:"rule id"`
}

func (s *vaultTools) forget(_ context.Context, _ *sdkmcp.CallToolRequest, args forgetArgs) (*sdkmcp.CallToolResult, any, error) {
	res, err := s.v.Forget(args.ID)
	if err != nil {
		return toolErr(err), nil, nil
	}
	return jsonOK(res)
}

type getArgs struct {
	ID string `json:"id" jsonschema:"rule id"`
}

func (s *vaultTools) get(_ context.Context, _ *sdkmcp.CallToolRequest, args getArgs) (*sdkmcp.CallToolResult, any, error) {
	rec, err := s.v.Get(args.ID)
	if err != nil {
		return toolErr(err), nil, nil
	}
	return jsonOK(rec)
}

type listArgs struct {
	Status        string  `json:"status,omitempty"`
	Scope         string  `json:"scope,omitempty" jsonschema:"comma-separated scope tags (AND)"`
	Query         string  `json:"query,omitempty"`
	MinConfidence float64 `json:"min_confidence,omitempty"`
	Since         string  `json:"since,omitempty" jsonschema:"YYYY-MM-DD or RFC3339"`
	Limit         int     `json:"limit,omitempty"`
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
	Limit int `json:"limit,omitempty"`
}

func (s *vaultTools) show(_ context.Context, _ *sdkmcp.CallToolRequest, args showArgs) (*sdkmcp.CallToolResult, any, error) {
	recs, err := s.v.Show(args.Limit)
	if err != nil {
		return toolErr(err), nil, nil
	}
	return jsonOK(recs)
}

type sweepArgs struct {
	DecayDays        int     `json:"decay_days,omitempty"`
	DecayAmount      float64 `json:"decay_amount,omitempty"`
	DormantThreshold float64 `json:"dormant_threshold,omitempty"`
}

func (s *vaultTools) sweep(_ context.Context, _ *sdkmcp.CallToolRequest, args sweepArgs) (*sdkmcp.CallToolResult, any, error) {
	res, err := s.v.Sweep(args.DecayDays, args.DecayAmount, args.DormantThreshold)
	if err != nil {
		return toolErr(err), nil, nil
	}
	return jsonOK(res)
}

type vizArgs struct {
	Out              string `json:"out,omitempty" jsonschema:"output path"`
	Format           string `json:"format,omitempty" jsonschema:"html, mermaid, or notes"`
	IncludeArchived  bool   `json:"include_archived,omitempty"`
}

func (s *vaultTools) viz(_ context.Context, _ *sdkmcp.CallToolRequest, args vizArgs) (*sdkmcp.CallToolResult, any, error) {
	format := args.Format
	if format == "" {
		format = "html"
	}
	res, err := s.v.Viz(args.Out, format, args.IncludeArchived)
	if err != nil {
		return toolErr(err), nil, nil
	}
	return jsonOK(res)
}
