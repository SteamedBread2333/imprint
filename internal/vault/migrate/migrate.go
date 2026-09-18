// Package migrate imports legacy markdown shards into vault.db.
package migrate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/SteamedBread2333/imprint/internal/vault/legacy"
	"github.com/SteamedBread2333/imprint/internal/vault/model"
	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

// ShardStat is one parsed shard file.
type ShardStat struct {
	Path  string `json:"path"`
	Rules int    `json:"rules"`
}

// Result is the outcome of a shard import.
type Result struct {
	Shards       []ShardStat `json:"shards"`
	FromMarkdown int         `json:"from_markdown"`
	KeptExisting int         `json:"kept_existing"`
	Total        int         `json:"total"`
	Active       int         `json:"active"`
	Dormant      int         `json:"dormant"`
	Superseded   int         `json:"superseded"`
	Imported     bool        `json:"imported"`
	DryRun       bool        `json:"dry_run"`
}

// ImportShards reads imprint-*.md under vaultDir (and vaultDir/archive), merges
// with existing vault.db rules (markdown wins on id conflict), and imports.
func ImportShards(v *imprint.Vault, dryRun bool) (Result, error) {
	dir := v.Dir
	shardPaths, err := discoverShards(dir)
	if err != nil {
		return Result{}, err
	}
	if len(shardPaths) == 0 {
		return Result{DryRun: dryRun}, fmt.Errorf("no imprint-*.md shards found under %s", dir)
	}

	byID := map[string]*imprint.Record{}
	res := Result{DryRun: dryRun}
	for _, path := range shardPaths {
		recs, err := legacy.ParseShardFile(path)
		if err != nil {
			return Result{}, err
		}
		rel, _ := filepath.Rel(dir, path)
		if rel == "" {
			rel = filepath.Base(path)
		}
		res.Shards = append(res.Shards, ShardStat{Path: rel, Rules: len(recs)})
		for _, r := range recs {
			byID[r.ID] = toImprintRecord(r)
			res.FromMarkdown++
		}
	}

	var buf bytes.Buffer
	if err := v.ExportJSON(&buf); err != nil {
		return Result{}, err
	}
	var existing []imprint.Record
	if buf.Len() > 0 {
		if err := json.Unmarshal(buf.Bytes(), &existing); err != nil {
			return Result{}, err
		}
	}
	for i := range existing {
		r := existing[i]
		if _, ok := byID[r.ID]; ok {
			continue
		}
		byID[r.ID] = &r
		res.KeptExisting++
	}

	recs := make([]*imprint.Record, 0, len(byID))
	for _, r := range byID {
		recs = append(recs, r)
		switch r.Status {
		case imprint.StatusActive:
			res.Active++
		case imprint.StatusDormant:
			res.Dormant++
		default:
			res.Superseded++
		}
	}
	sort.Slice(recs, func(i, j int) bool { return recs[i].ID < recs[j].ID })
	res.Total = len(recs)

	if dryRun {
		return res, nil
	}
	if err := v.ImportRecords(recs); err != nil {
		return Result{}, err
	}
	res.Imported = true
	return res, nil
}

func discoverShards(vaultDir string) ([]string, error) {
	var paths []string
	for _, dir := range []string{vaultDir, filepath.Join(vaultDir, "archive")} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasPrefix(e.Name(), "imprint-") || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func toImprintRecord(r *model.Record) *imprint.Record {
	if r == nil {
		return nil
	}
	out := imprint.Record(*r)
	return &out
}
