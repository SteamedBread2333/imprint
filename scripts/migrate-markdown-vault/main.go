// One-off: import legacy imprint-NNNN.md shards into vault.db.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

func main() {
	vaultDir := flag.String("vault", ".imprint/memory", "vault directory")
	dryRun := flag.Bool("dry-run", false, "parse and report only; do not write vault.db")
	flag.Parse()

	abs, err := filepath.Abs(*vaultDir)
	if err != nil {
		fatal(err)
	}

	var shardPaths []string
	for _, dir := range []string{abs, filepath.Join(abs, "archive")} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			fatal(err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasPrefix(e.Name(), "imprint-") || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			shardPaths = append(shardPaths, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(shardPaths)
	if len(shardPaths) == 0 {
		fmt.Println("no imprint-*.md shards found")
		return
	}

	byID := map[string]*imprint.Record{}
	var fromMarkdown int
	for _, path := range shardPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			fatal(fmt.Errorf("%s: %w", path, err))
		}
		recs, err := unmarshalShard(data)
		if err != nil {
			fatal(fmt.Errorf("%s: %w", path, err))
		}
		fmt.Printf("%s: %d rules\n", filepath.Base(path), len(recs))
		for _, r := range recs {
			if prev, ok := byID[r.ID]; ok {
				fmt.Fprintf(os.Stderr, "warning: duplicate id %s (keeping %s, dropping %s)\n", r.ID, path, prev.Path)
			}
			r.Path = ""
			byID[r.ID] = r
			fromMarkdown++
		}
	}

	v, err := imprint.Open(abs)
	if err != nil {
		fatal(err)
	}
	var buf bytes.Buffer
	if err := v.ExportJSON(&buf); err != nil {
		fatal(err)
	}
	var existing []imprint.Record
	if buf.Len() > 0 {
		if err := json.Unmarshal(buf.Bytes(), &existing); err != nil {
			fatal(err)
		}
	}
	keptExisting := 0
	for i := range existing {
		r := existing[i]
		if _, ok := byID[r.ID]; ok {
			continue
		}
		byID[r.ID] = &r
		keptExisting++
	}

	recs := make([]*imprint.Record, 0, len(byID))
	for _, r := range byID {
		recs = append(recs, r)
	}
	sort.Slice(recs, func(i, j int) bool { return recs[i].ID < recs[j].ID })

	active, dormant, superseded := 0, 0, 0
	for _, r := range recs {
		switch r.Status {
		case imprint.StatusActive:
			active++
		case imprint.StatusDormant:
			dormant++
		default:
			superseded++
		}
	}

	fmt.Printf("markdown parsed: %d rules from %d shard(s)\n", fromMarkdown, len(shardPaths))
	fmt.Printf("kept from existing vault.db: %d\n", keptExisting)
	fmt.Printf("total to import: %d (active=%d dormant=%d superseded=%d)\n", len(recs), active, dormant, superseded)

	if *dryRun {
		fmt.Println("dry-run: vault.db unchanged")
		return
	}

	if err := v.ImportRecords(recs); err != nil {
		fatal(err)
	}
	fmt.Println("imported into vault.db")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
