package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SteamedBread2333/imprint/internal/telemetry"
	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

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

func (a *App) cmdImport(g globals, args []string) int {
	fs := newFlags()
	pos, err := fs.parse(args)
	if err != nil {
		if err == errHelp {
			fmt.Fprint(a.out(), commandHelp("import"))
			return 0
		}
		return a.fail(g.json, err)
	}
	v, err := a.openVault(g)
	if err != nil {
		return a.fail(g.json, err)
	}
	defer v.Close()
	path := ""
	if len(pos) > 0 {
		path = pos[0]
	} else {
		exportDir := filepath.Join(v.Dir, imprint.ExportDirName)
		for _, name := range []string{"vault.json", "vault.jsonl"} {
			cand := filepath.Join(exportDir, name)
			if st, err := os.Stat(cand); err == nil && !st.IsDir() {
				path = cand
				break
			}
		}
		if path == "" {
			return a.fail(g.json, fmt.Errorf("usage: imprint import [FILE] (no %s/vault.json or vault.jsonl)", imprint.ExportDirName))
		}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return a.fail(g.json, err)
	}
	recs, err := parseExportedRecords(raw)
	if err != nil {
		return a.fail(g.json, err)
	}
	res, err := v.ImportRecords(recs)
	if err != nil {
		return a.fail(g.json, err)
	}
	if g.json {
		return a.fail(false, a.writeJSON(map[string]any{
			"imported": len(recs),
			"path":     path,
			"backfill": res,
		}))
	}
	c := a.console()
	c.Heading("import")
	c.Done("imported %d rules from %s", len(recs), path)
	printBackfillHint(c, res)
	c.blank()
	return 0
}

func parseExportedRecords(raw []byte) ([]*imprint.Record, error) {
	trim := bytes.TrimSpace(raw)
	if len(trim) == 0 {
		return nil, fmt.Errorf("import file is empty")
	}
	if trim[0] == '[' {
		var recs []*imprint.Record
		if err := json.Unmarshal(trim, &recs); err != nil {
			return nil, fmt.Errorf("parse json export: %w", err)
		}
		return recs, nil
	}
	dec := json.NewDecoder(bytes.NewReader(trim))
	var recs []*imprint.Record
	for {
		var rec imprint.Record
		if err := dec.Decode(&rec); err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("parse jsonl export: %w", err)
		}
		cp := rec
		recs = append(recs, &cp)
	}
	return recs, nil
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
