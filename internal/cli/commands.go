package cli

import (
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

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
	res, err := v.Add(claim, splitCSV(*scope), *text, *conf)
	if err != nil {
		return a.fail(g.json, err)
	}
	if g.json {
		return a.fail(false, a.writeJSON(res))
	}
	fmt.Fprintf(a.out(), "added %s  confidence=%.2f\n%s\n", res.ID, res.Confidence, res.Path)
	return 0
}

func (a *App) cmdFind(g globals, args []string) int {
	fs := newFlags()
	scope := fs.String("scope", "")
	query := fs.String("query", "")
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
	hits, err := v.Find(splitCSV(*scope), *query, *topK)
	if err != nil {
		return a.fail(g.json, err)
	}
	if g.json {
		return a.fail(false, a.writeJSON(hits))
	}
	if len(hits) == 0 {
		fmt.Fprintln(a.out(), "no matching imprints")
		return 0
	}
	w := tabwriter.NewWriter(a.out(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSCORE\tCONF\tSCOPE\tCLAIM")
	for _, h := range hits {
		fmt.Fprintf(w, "%s\t%.2f\t%.2f\t%s\t%s\n", h.ID, h.Score, h.Confidence, strings.Join(h.Scope, ","), h.Title)
	}
	_ = w.Flush()
	return 0
}

func (a *App) cmdReinforce(g globals, args []string) int {
	fs := newFlags()
	evidence := fs.String("evidence", "")
	pos, err := fs.parse(args)
	if err != nil {
		if err == errHelp {
			fmt.Fprint(a.out(), commandHelp("reinforce"))
			return 0
		}
		return a.fail(g.json, err)
	}
	if len(pos) != 1 {
		return a.fail(g.json, fmt.Errorf("usage: imprint reinforce ID [--evidence TEXT]"))
	}
	v, err := a.openVault(g)
	if err != nil {
		return a.fail(g.json, err)
	}
	res, err := v.Reinforce(pos[0], *evidence)
	if err != nil {
		return a.fail(g.json, err)
	}
	if g.json {
		return a.fail(false, a.writeJSON(res))
	}
	fmt.Fprintf(a.out(), "reinforced %s  confidence=%.2f  count=%d\n", res.ID, res.Confidence, res.ReinforcementCount)
	return 0
}

func (a *App) cmdSupersede(g globals, args []string) int {
	fs := newFlags()
	claim := fs.String("claim", "")
	scope := fs.String("scope", "")
	reason := fs.String("reason", "")
	text := fs.String("text", "")
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
	res, err := v.Supersede(pos[0], *claim, splitCSV(*scope), *reason, *text)
	if err != nil {
		return a.fail(g.json, err)
	}
	if g.json {
		return a.fail(false, a.writeJSON(res))
	}
	fmt.Fprintf(a.out(), "superseded %s → %s\n", res.SupersededOldID, res.ID)
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
	fmt.Fprintf(a.out(), "forgot %s\n", pos[0])
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
	w := tabwriter.NewWriter(a.out(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tCONF\tSTATUS\tCLAIM")
	for _, it := range items {
		fmt.Fprintf(w, "%s\t%.2f\t%s\t%s\n", it.ID, it.Confidence, it.Status, it.Title)
	}
	_ = w.Flush()
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
	fmt.Fprintf(a.out(), "%s  [%.2f %s]\n%s\n scope: %s\n path: %s\n", rec.ID, rec.Confidence, rec.Status, rec.Claim, strings.Join(rec.Scope, ", "), rec.Path)
	if len(rec.ReferencedBy) > 0 {
		fmt.Fprint(a.out(), " referenced_by:")
		for _, b := range rec.ReferencedBy {
			fmt.Fprintf(a.out(), " %s(%s)", b.ID, b.Kind)
		}
		fmt.Fprintln(a.out())
	}
	for _, e := range rec.EvidenceLog {
		fmt.Fprintf(a.out(), "  %s  %s  %s\n", e.At.Format("2006-01-02"), e.Kind, e.Text)
	}
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
	fmt.Fprintf(a.out(), "decayed %d, archived %d\n", res.Decayed, res.Archived)
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
	w := tabwriter.NewWriter(a.out(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tCONF\tSTATUS\tSCOPE\tCLAIM")
	for _, r := range recs {
		fmt.Fprintf(w, "%s\t%.2f\t%s\t%s\t%s\n", r.ID, r.Confidence, r.Status, strings.Join(r.Scope, ","), r.Claim)
	}
	_ = w.Flush()
	return 0
}

func (a *App) cmdViz(g globals, args []string) int {
	fs := newFlags()
	out := fs.String("out", "")
	format := fs.String("format", "html")
	archived := fs.Bool("include-archived", false)
	_, err := fs.parse(args)
	if err != nil {
		if err == errHelp {
			fmt.Fprint(a.out(), commandHelp("viz"))
			return 0
		}
		return a.fail(g.json, err)
	}
	v, err := a.openVault(g)
	if err != nil {
		return a.fail(g.json, err)
	}
	res, err := v.Viz(*out, *format, *archived)
	if err != nil {
		return a.fail(g.json, err)
	}
	if g.json {
		return a.fail(false, a.writeJSON(res))
	}
	if res.Mermaid != "" && res.Path == "" {
		fmt.Fprint(a.out(), res.Mermaid)
		return 0
	}
	fmt.Fprintf(a.out(), "wrote %s  (%d rules, %d bytes)\n", res.Path, res.RulesCount, res.SizeBytes)
	return 0
}

func (a *App) cmdExport(g globals, args []string) int {
	fs := newFlags()
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
	if err := v.ExportJSON(a.out()); err != nil {
		return a.fail(g.json, err)
	}
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
	fmt.Fprintln(a.out(), "vault cleared")
	return 0
}
