package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SteamedBread2333/imprint/internal/argx"
	"github.com/SteamedBread2333/imprint/internal/linking"
	"github.com/SteamedBread2333/imprint/internal/plugin"
	"github.com/SteamedBread2333/imprint/internal/shelves"
	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

func (a *App) cmdAdd(g globals, args []string) int {
	fs := newFlags()
	scope := fs.String("scope", "")
	text := fs.String("text", "")
	conf := fs.Float("confidence", 0)
	queryLocal := fs.String("query-local", "")
	conflicts := fs.String("conflicts", "")
	sourcesFlag := fs.String("sources", "")
	pos, err := fs.parse(args)
	if err != nil {
		if err == errHelp {
			fmt.Fprint(a.out(), commandHelp("add"))
			return 0
		}
		return a.fail(g.json, err)
	}
	sources, err := parseSourcesFlag(*sourcesFlag)
	if err != nil {
		return a.fail(g.json, err)
	}
	claim := strings.TrimSpace(strings.Join(pos, " "))
	v, err := a.openVault(g)
	if err != nil {
		return a.fail(g.json, err)
	}
	res, err := v.AddRecord(claim, argx.SplitCSV(*scope), *text, *conf, nil, nil, argx.SplitCSV(*conflicts), sources, *queryLocal)
	if err != nil {
		return a.fail(g.json, err)
	}
	if g.json {
		return a.fail(false, a.writeJSON(res))
	}
	c := a.console()
	c.Heading("add")
	c.Done("added %s  ·  confidence %.2f", res.ID, res.Confidence)
	c.KV("path", res.Path)
	if len(res.Similar) > 0 {
		items := make([]similarItem, len(res.Similar))
		for i, s := range res.Similar {
			items[i] = similarItem{ID: s.ID, Score: s.Score, Claim: s.Claim}
		}
		c.SimilarAdvisory(items)
	}
	c.blank()
	return 0
}

func (a *App) cmdFind(g globals, args []string) int {
	fs := newFlags()
	scope := fs.String("scope", "")
	query := fs.String("query", "")
	queryLocal := fs.String("query-local", "")
	pathsFlag := fs.String("paths", "")
	topK := fs.Int("top-k", imprint.DefaultTopK)
	pos, err := fs.parse(args)
	if err != nil {
		if err == errHelp {
			fmt.Fprint(a.out(), commandHelp("find"))
			return 0
		}
		return a.fail(g.json, err)
	}
	if *query == "" && len(pos) > 0 {
		*query = strings.Join(pos, " ")
	}
	v, err := a.openVault(g)
	if err != nil {
		return a.fail(g.json, err)
	}
	svc, err := shelvesForVault(v)
	if err != nil {
		return a.fail(g.json, err)
	}
	out, err := linking.Find(v, svc, linking.FindRequest{
		Scope: argx.SplitCSV(*scope), Query: *query, QueryLocal: *queryLocal,
		TopK: *topK, Paths: argx.SplitCSV(*pathsFlag),
	})
	if err != nil {
		return a.fail(g.json, err)
	}
	if g.json {
		switch {
		case out.Docs != nil:
			return a.fail(false, a.writeJSON(out.Docs))
		case out.Rules != nil:
			return a.fail(false, a.writeJSON(out.Rules))
		}
		return a.fail(false, a.writeJSON(out.Hits))
	}
	c := a.console()
	c.Heading("find")
	hits := out.RuleHits()
	if len(hits) == 0 {
		c.Empty("no matching imprints")
	}
	for _, h := range hits {
		c.RuleFind(h.ID, h.Score, h.Confidence, strings.Join(h.Scope, ","), h.Title)
	}
	if out.Docs != nil && len(out.Docs.Documents) > 0 {
		c.Section("documents")
		for _, d := range out.Docs.Documents {
			c.DocFind(d.Path, d.Heading, d.Score, d.Snippet)
		}
	}
	c.blank()
	return 0
}

// parseSourcesFlag decodes --sources, the same JSON array the MCP tools take:
// [{"path":"docs/x.md","heading":"Intro","chunk":"..."}]. Empty means none.
func parseSourcesFlag(s string) ([]imprint.DocRef, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	var refs []imprint.DocRef
	if err := json.Unmarshal([]byte(s), &refs); err != nil {
		return nil, fmt.Errorf(`invalid --sources (want JSON like [{"path":"docs/x.md","heading":"Intro"}]): %w`, err)
	}
	for i, r := range refs {
		if strings.TrimSpace(r.Path) == "" {
			return nil, fmt.Errorf("invalid --sources: item %d has empty path", i)
		}
	}
	return refs, nil
}

// shelvesForVault loads the shelves index for the project owning v, or nil
// when the project has no imprint.yaml or shelves is disabled.
func shelvesForVault(v *imprint.Vault) (*shelves.Service, error) {
	cfgPath, ok := plugin.ResolveConfigPathForVault(v.Dir)
	if !ok {
		return nil, nil
	}
	pcfg, err := plugin.Load(cfgPath)
	if err != nil {
		return nil, err
	}
	scfg := shelves.ConfigFrom(pcfg)
	if !scfg.Enabled {
		return nil, nil
	}
	return shelves.New(scfg)
}

func (a *App) cmdReinforce(g globals, args []string) int {
	fs := newFlags()
	evidence := fs.String("evidence", "")
	queryLocal := fs.String("query-local", "")
	pos, err := fs.parse(args)
	if err != nil {
		if err == errHelp {
			fmt.Fprint(a.out(), commandHelp("reinforce"))
			return 0
		}
		return a.fail(g.json, err)
	}
	if len(pos) != 1 {
		return a.fail(g.json, fmt.Errorf("usage: imprint reinforce ID --evidence TEXT"))
	}
	v, err := a.openVault(g)
	if err != nil {
		return a.fail(g.json, err)
	}
	res, err := v.ReinforceQueryLocal(pos[0], *evidence, *queryLocal)
	if err != nil {
		return a.fail(g.json, err)
	}
	if g.json {
		return a.fail(false, a.writeJSON(res))
	}
	c := a.console()
	c.Heading("reinforce")
	c.Done("reinforced %s  ·  confidence %.2f  ·  count %d", res.ID, res.Confidence, res.ReinforcementCount)
	c.blank()
	return 0
}

func (a *App) cmdSupersede(g globals, args []string) int {
	fs := newFlags()
	claim := fs.String("claim", "")
	scope := fs.String("scope", "")
	reason := fs.String("reason", "")
	text := fs.String("text", "")
	queryLocal := fs.String("query-local", "")
	sourcesFlag := fs.String("sources", "")
	pos, err := fs.parse(args)
	if err != nil {
		if err == errHelp {
			fmt.Fprint(a.out(), commandHelp("supersede"))
			return 0
		}
		return a.fail(g.json, err)
	}
	if len(pos) != 1 {
		return a.fail(g.json, fmt.Errorf("usage: imprint supersede OLD_ID --claim NEW --scope tag,tag"))
	}
	sources, err := parseSourcesFlag(*sourcesFlag)
	if err != nil {
		return a.fail(g.json, err)
	}
	v, err := a.openVault(g)
	if err != nil {
		return a.fail(g.json, err)
	}
	res, err := v.SupersedeWithSources(pos[0], *claim, argx.SplitCSV(*scope), *reason, *text, sources, *queryLocal)
	if err != nil {
		return a.fail(g.json, err)
	}
	if g.json {
		return a.fail(false, a.writeJSON(res))
	}
	c := a.console()
	c.Heading("supersede")
	c.Done("superseded %s → %s", res.SupersededOldID, res.ID)
	c.blank()
	return 0
}

func (a *App) cmdForget(g globals, args []string) int {
	fs := newFlags()
	pos, err := fs.parse(args)
	if err != nil {
		if err == errHelp {
			fmt.Fprint(a.out(), commandHelp("forget"))
			return 0
		}
		return a.fail(g.json, err)
	}
	if len(pos) != 1 {
		return a.fail(g.json, fmt.Errorf("usage: imprint forget ID"))
	}
	v, err := a.openVault(g)
	if err != nil {
		return a.fail(g.json, err)
	}
	res, err := v.Forget(pos[0])
	if err != nil {
		return a.fail(g.json, err)
	}
	if g.json {
		return a.fail(false, a.writeJSON(res))
	}
	c := a.console()
	c.Heading("forget")
	c.Done("forgot %s", pos[0])
	c.blank()
	return 0
}

func (a *App) cmdLink(g globals, args []string) int {
	fs := newFlags()
	related := fs.String("related", "")
	conflicts := fs.String("conflicts", "")
	pos, err := fs.parse(args)
	if err != nil {
		if err == errHelp {
			fmt.Fprint(a.out(), commandHelp("link"))
			return 0
		}
		return a.fail(g.json, err)
	}
	if len(pos) != 1 {
		return a.fail(g.json, fmt.Errorf("usage: imprint link ID [--related id,id] [--conflicts id,id]"))
	}
	v, err := a.openVault(g)
	if err != nil {
		return a.fail(g.json, err)
	}
	res, err := v.Link(pos[0], argx.SplitCSV(*related), argx.SplitCSV(*conflicts))
	if err != nil {
		return a.fail(g.json, err)
	}
	if g.json {
		return a.fail(false, a.writeJSON(res))
	}
	c := a.console()
	c.Heading("link")
	c.Done("linked %s  ·  related +%d  ·  conflicts +%d", res.ID, res.RelatedAdded, res.ConflictsAdded)
	c.blank()
	return 0
}

func (a *App) cmdList(g globals, args []string) int {
	fs := newFlags()
	status := fs.String("status", "")
	scope := fs.String("scope", "")
	query := fs.String("query", "")
	minConf := fs.Float("min-confidence", 0)
	since := fs.String("since", "")
	limit := fs.Int("limit", 0)
	_, err := fs.parse(args)
	if err != nil {
		if err == errHelp {
			fmt.Fprint(a.out(), commandHelp("list"))
			return 0
		}
		return a.fail(g.json, err)
	}
	when, err := argx.ParseSince(*since, "--since")
	if err != nil {
		return a.fail(g.json, err)
	}
	v, err := a.openVault(g)
	if err != nil {
		return a.fail(g.json, err)
	}
	items, err := v.ListFilter(imprint.ListFilter{
		Status:        *status,
		Scope:         argx.SplitCSV(*scope),
		MinConfidence: *minConf,
		Query:         *query,
		Since:         when,
		Limit:         *limit,
	})
	if err != nil {
		return a.fail(g.json, err)
	}
	if g.json {
		return a.fail(false, a.writeJSON(items))
	}
	c := a.console()
	c.Heading("list")
	if len(items) == 0 {
		c.Empty("no rules match")
		c.blank()
		return 0
	}
	for _, it := range items {
		c.RuleBrief(it.ID, it.Confidence, string(it.Status), "", it.Title)
	}
	c.blank()
	return 0
}

func (a *App) cmdGet(g globals, args []string) int {
	fs := newFlags()
	pos, err := fs.parse(args)
	if err != nil {
		if err == errHelp {
			fmt.Fprint(a.out(), commandHelp("get"))
			return 0
		}
		return a.fail(g.json, err)
	}
	if len(pos) != 1 {
		return a.fail(g.json, fmt.Errorf("usage: imprint get ID"))
	}
	v, err := a.openVault(g)
	if err != nil {
		return a.fail(g.json, err)
	}
	rec, err := v.Get(pos[0])
	if err != nil {
		return a.fail(g.json, err)
	}
	if g.json {
		return a.fail(false, a.writeJSON(rec))
	}
	c := a.console()
	c.Heading(rec.ID)
	c.KV("confidence", fmt.Sprintf("%.2f", rec.Confidence))
	c.KV("status", string(rec.Status))
	c.KV("scope", strings.Join(rec.Scope, ", "))
	c.KV("path", rec.Path)
	c.Section("claim")
	c.ruleClaim(rec.Claim)
	if len(rec.ReferencedBy) > 0 {
		var refs []string
		for _, b := range rec.ReferencedBy {
			refs = append(refs, fmt.Sprintf("%s (%s)", b.ID, b.Kind))
		}
		c.KV("referenced_by", strings.Join(refs, ", "))
	}
	if len(rec.EvidenceLog) > 0 {
		c.Section("evidence")
		for _, e := range rec.EvidenceLog {
			c.KV(e.At.Format("2006-01-02"), fmt.Sprintf("%s  ·  %s", e.Kind, e.Text))
		}
	}
	c.blank()
	return 0
}

func (a *App) cmdSweep(g globals, args []string) int {
	fs := newFlags()
	days := fs.Int("decay-days", 0)
	amount := fs.Float("decay-amount", 0)
	thresh := fs.Float("dormant-threshold", 0)
	_, err := fs.parse(args)
	if err != nil {
		if err == errHelp {
			fmt.Fprint(a.out(), commandHelp("sweep"))
			return 0
		}
		return a.fail(g.json, err)
	}
	v, err := a.openVault(g)
	if err != nil {
		return a.fail(g.json, err)
	}
	res, err := v.Sweep(*days, *amount, *thresh)
	if err != nil {
		return a.fail(g.json, err)
	}
	if g.json {
		return a.fail(false, a.writeJSON(res))
	}
	c := a.console()
	c.Heading("sweep")
	c.Done("decayed %d  ·  dormant %d", res.Decayed, res.Archived)
	c.blank()
	return 0
}

func (a *App) cmdShow(g globals, args []string) int {
	fs := newFlags()
	limit := fs.Int("limit", 0)
	format := fs.String("format", "table")
	_, err := fs.parse(args)
	if err != nil {
		if err == errHelp {
			fmt.Fprint(a.out(), commandHelp("show"))
			return 0
		}
		return a.fail(g.json, err)
	}
	v, err := a.openVault(g)
	if err != nil {
		return a.fail(g.json, err)
	}
	recs, err := v.Show(*limit)
	if err != nil {
		return a.fail(g.json, err)
	}
	if g.json || *format == "json" {
		return a.fail(false, a.writeJSON(recs))
	}
	c := a.console()
	c.Heading("show")
	if len(recs) == 0 {
		c.Empty("vault is empty")
		c.blank()
		return 0
	}
	for _, r := range recs {
		c.RuleBrief(r.ID, r.Confidence, string(r.Status), strings.Join(r.Scope, ","), r.Claim)
	}
	c.blank()
	return 0
}
