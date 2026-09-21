package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/SteamedBread2333/imprint/internal/telemetry"
	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

type flagSet struct {
	bools  map[string]*bool
	strs   map[string]*string
	ints   map[string]*int
	floats map[string]*float64
}

func newFlags() *flagSet {
	return &flagSet{
		bools:  map[string]*bool{},
		strs:   map[string]*string{},
		ints:   map[string]*int{},
		floats: map[string]*float64{},
	}
}

func (f *flagSet) Bool(name string, def bool) *bool {
	v := def
	f.bools[name] = &v
	return &v
}

func (f *flagSet) String(name, def string) *string {
	v := def
	f.strs[name] = &v
	return &v
}

func (f *flagSet) Int(name string, def int) *int {
	v := def
	f.ints[name] = &v
	return &v
}

func (f *flagSet) Float(name string, def float64) *float64 {
	v := def
	f.floats[name] = &v
	return &v
}

func (f *flagSet) parse(args []string) ([]string, error) {
	var pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		if a == "-h" || a == "--help" {
			return nil, errHelp
		}
		if !strings.HasPrefix(a, "--") {
			pos = append(pos, a)
			continue
		}
		name := strings.TrimPrefix(a, "--")
		raw := ""
		hasVal := false
		if j := strings.IndexByte(name, '='); j >= 0 {
			raw = name[j+1:]
			name = name[:j]
			hasVal = true
		}
		if _, ok := f.bools[name]; ok {
			if !hasVal {
				raw = "true"
			}
			b, err := strconv.ParseBool(raw)
			if err != nil {
				return nil, fmt.Errorf("flag --%s: %w", name, err)
			}
			*f.bools[name] = b
			continue
		}
		if !hasVal {
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("flag --%s needs a value", name)
			}
			raw = args[i]
		}
		switch {
		case f.strs[name] != nil:
			*f.strs[name] = raw
		case f.ints[name] != nil:
			n, err := strconv.Atoi(raw)
			if err != nil {
				return nil, fmt.Errorf("flag --%s: %w", name, err)
			}
			*f.ints[name] = n
		case f.floats[name] != nil:
			n, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				return nil, fmt.Errorf("flag --%s: %w", name, err)
			}
			*f.floats[name] = n
		default:
			return nil, fmt.Errorf("unknown flag --%s", name)
		}
	}
	return pos, nil
}

type helpError struct{}

func (helpError) Error() string { return "help" }

var errHelp = helpError{}

func (a *App) cmdAdd(g globals, args []string) int {
	fs := newFlags()
	scope := fs.String("scope", "")
	text := fs.String("text", "")
	conf := fs.Float("confidence", 0)
	queryLocal := fs.String("query-local", "")
	pos, err := fs.parse(args)
	if err != nil {
		if err == errHelp {
			fmt.Fprint(a.out(), commandHelp("add"))
			return 0
		}
		return a.fail(g.json, err)
	}
	claim := strings.TrimSpace(strings.Join(pos, " "))
	v, err := a.openVault(g)
	if err != nil {
		return a.fail(g.json, err)
	}
	res, err := v.AddRecord(claim, splitCSV(*scope), *text, *conf, nil, nil, nil, nil, *queryLocal)
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
	c.blank()
	return 0
}

func (a *App) cmdFind(g globals, args []string) int {
	fs := newFlags()
	scope := fs.String("scope", "")
	query := fs.String("query", "")
	queryLocal := fs.String("query-local", "")
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
	hits, err := v.FindMerged(splitCSV(*scope), *query, *queryLocal, *topK)
	if err != nil {
		return a.fail(g.json, err)
	}
	if g.json {
		return a.fail(false, a.writeJSON(hits))
	}
	c := a.console()
	c.Heading("find")
	if len(hits) == 0 {
		c.Empty("no matching imprints")
		c.blank()
		return 0
	}
	for _, h := range hits {
		c.RuleFind(h.ID, h.Score, h.Confidence, strings.Join(h.Scope, ","), h.Title)
	}
	c.blank()
	return 0
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
	v, err := a.openVault(g)
	if err != nil {
		return a.fail(g.json, err)
	}
	res, err := v.SupersedeWithSources(pos[0], *claim, splitCSV(*scope), *reason, *text, nil, *queryLocal)
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
	when, err := parseSince(*since)
	if err != nil {
		return a.fail(g.json, err)
	}
	v, err := a.openVault(g)
	if err != nil {
		return a.fail(g.json, err)
	}
	items, err := v.ListFilter(imprint.ListFilter{
		Status:        *status,
		Scope:         splitCSV(*scope),
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

func parseSince(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid --since %q (use YYYY-MM-DD or RFC3339)", s)
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

func (a *App) cmdExport(g globals, args []string) int {
	fs := newFlags()
	format := fs.String("format", "json")
	_, err := fs.parse(args)
	if err != nil {
		if err == errHelp {
			fmt.Fprint(a.out(), commandHelp("export"))
			return 0
		}
		return a.fail(g.json, err)
	}
	v, err := a.openVault(g)
	if err != nil {
		return a.fail(g.json, err)
	}
	exportDir := filepath.Join(v.Dir, imprint.ExportDirName)
	if err := os.MkdirAll(exportDir, 0o755); err != nil {
		return a.fail(g.json, err)
	}
	var (
		name     string
		exportFn func(io.Writer) error
	)
	switch strings.ToLower(strings.TrimSpace(*format)) {
	case "json":
		name, exportFn = "vault.json", v.ExportJSON
	case "jsonl":
		name, exportFn = "vault.jsonl", v.ExportJSONL
	default:
		return a.fail(g.json, fmt.Errorf("invalid export format %q (use json or jsonl)", *format))
	}
	exportPath := filepath.Join(exportDir, name)
	f, err := os.Create(exportPath)
	if err != nil {
		return a.fail(g.json, err)
	}
	if err := exportFn(f); err != nil {
		_ = f.Close()
		return a.fail(g.json, err)
	}
	if err := f.Close(); err != nil {
		return a.fail(g.json, err)
	}
	if g.json {
		if err := exportFn(a.out()); err != nil {
			return a.fail(g.json, err)
		}
		return 0
	}
	c := a.console()
	c.Heading("export")
	c.Done("wrote %s", exportPath)
	c.blank()
	return 0
}

func (a *App) cmdReport(g globals, args []string) int {
	fs := newFlags()
	days := fs.Int("days", 30)
	if _, err := fs.parse(args); err != nil {
		if err == errHelp {
			fmt.Fprint(a.out(), commandHelp("report"))
			return 0
		}
		return a.fail(g.json, err)
	}
	v, err := a.openVault(g)
	if err != nil {
		return a.fail(g.json, err)
	}
	defer v.Close()
	report, err := v.Report(*days)
	if err != nil {
		return a.fail(g.json, err)
	}
	tel, err := telemetry.Summarize(
		filepath.Join(v.Dir, imprint.StateDirName, "telemetry"),
		a.now().Add(-time.Duration(*days)*24*time.Hour),
	)
	if err != nil {
		return a.fail(g.json, err)
	}
	payload := struct {
		*imprint.ReportResult
		Telemetry telemetry.Summary `json:"telemetry"`
	}{ReportResult: report, Telemetry: tel}
	if g.json {
		_ = a.writeJSON(payload)
		return 0
	}
	c := a.console()
	c.Heading("report")
	fmt.Fprintf(a.out(), "  since          %s\n", report.Since.Format(time.RFC3339))
	fmt.Fprintf(a.out(), "  rules          %d\n", len(report.Rules))
	fmt.Fprintf(a.out(), "  duplicates     %d\n", len(report.Duplicates))
	fmt.Fprintf(a.out(), "  conflicts      %d\n", len(report.Conflicts))
	fmt.Fprintf(a.out(), "  telemetry      %d events, %.1fms average\n", tel.Events, tel.AvgLatency)
	for kind, count := range report.EventCounts {
		fmt.Fprintf(a.out(), "  %-14s %d\n", kind, count)
	}
	for _, rule := range report.Rules {
		if rule.Recommendation != "" {
			fmt.Fprintf(a.out(), "  review %-24s %s\n", rule.ID, rule.Recommendation)
		}
	}
	c.blank()
	return 0
}

func (a *App) cmdClear(g globals, args []string) int {
	fs := newFlags()
	confirm := fs.Bool("confirm", false)
	yes := fs.Bool("yes", false)
	_, err := fs.parse(args)
	if err != nil {
		if err == errHelp {
			fmt.Fprint(a.out(), commandHelp("clear"))
			return 0
		}
		return a.fail(g.json, err)
	}
	if !*confirm || !*yes {
		msg := "clear is irreversible; pass --confirm --yes to delete every rule in the vault"
		if g.json {
			return a.fail(true, fmt.Errorf("%s", msg))
		}
		fmt.Fprintln(a.errw(), "imprint:", msg)
		return 1
	}
	v, err := a.openVault(g)
	if err != nil {
		return a.fail(g.json, err)
	}
	if err := v.Clear(); err != nil {
		return a.fail(g.json, err)
	}
	if g.json {
		return a.fail(false, a.writeJSON(map[string]any{"success": true}))
	}
	c := a.console()
	c.Heading("clear")
	c.Done("vault cleared")
	c.blank()
	return 0
}
