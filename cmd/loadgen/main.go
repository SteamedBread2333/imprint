package main

// loadgen writes a synthetic SQLite vault for graph/find timing.

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

var langs = []string{"go", "python", "javascript", "typescript", "rust"}
var topics = []string{"naming", "testing", "error-handling", "api-design", "git", "cli", "build", "storage"}
var projs = []string{"proj:imprint", "proj:acme", "proj:web"}

func main() {
	n := flag.Int("n", 3000, "number of rules to write")
	vault := flag.String("vault", "loadvault", "vault directory (use a gitignored path; memory-test uses tests/loadvault)")
	flag.Parse()

	start := time.Now()
	if err := os.RemoveAll(*vault); err != nil {
		fatal(err)
	}

	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	recs := make([]*imprint.Record, 0, *n)
	daySeq := map[string]int{}

	for i := 0; i < *n; i++ {
		t := base.AddDate(0, 0, i/80)
		key := t.Format("2006-01-02")
		daySeq[key]++
		id := fmt.Sprintf("r-%s-%03d", key, daySeq[key])
		lang := langs[i%len(langs)]
		topic := topics[(i/len(langs))%len(topics)]
		scope := []string{lang, topic}
		if i%7 == 0 {
			scope = append(scope, projs[i%len(projs)])
		}
		conf := 0.35 + float64((i*17)%60)/100
		if conf > 0.95 {
			conf = 0.95
		}
		st := imprint.StatusActive
		switch {
		case i%23 == 0:
			st = imprint.StatusDormant
			if conf > 0.28 {
				conf = 0.28
			}
		case i%11 == 0:
			st = imprint.StatusSuperseded
		}
		rec := &imprint.Record{
			ID:                 id,
			Claim:              fmt.Sprintf("Prefer %s %s convention variant %d", lang, topic, i%50),
			Scope:              scope,
			Confidence:         conf,
			Status:             st,
			ReinforcementCount: i % 4,
			CreatedAt:          t,
			UpdatedAt:          t,
			LastTouchedAt:      t,
			Supersedes:         []string{},
			Related:            []string{},
			ConflictsWith:      []string{},
			EvidenceLog: []imprint.Evidence{{
				At:   t,
				Kind: imprint.EvidenceOriginal,
				Text: fmt.Sprintf("synthetic loadgen %d", i),
			}},
		}
		recs = append(recs, rec)
	}

	for i := 1; i < len(recs); i += 12 {
		prev := recs[i-1]
		cur := recs[i]
		cur.Supersedes = []string{prev.ID}
		cur.Related = []string{prev.ID}
		cur.Status = imprint.StatusActive
		prev.Status = imprint.StatusSuperseded
		prev.EvidenceLog = append(prev.EvidenceLog, imprint.Evidence{
			At:   cur.CreatedAt,
			Kind: imprint.EvidenceSupersede,
			Text: "superseded by " + cur.ID,
		})
	}
	for i := 20; i < len(recs); i += 40 {
		recs[i].Related = append(recs[i].Related, recs[i-3].ID)
	}
	for i := 30; i < len(recs); i += 90 {
		recs[i].ConflictsWith = []string{recs[i-5].ID}
	}

	v, err := imprint.Open(*vault)
	if err != nil {
		fatal(err)
	}
	if err := v.ImportRecords(recs); err != nil {
		fatal(err)
	}
	wrote := time.Since(start)

	graphStart := time.Now()
	g, err := v.Graph(true)
	if err != nil {
		fatal(err)
	}
	graphDur := time.Since(graphStart)

	findStart := time.Now()
	hits, err := v.Find([]string{"go", "naming"}, "", 5)
	if err != nil {
		fatal(err)
	}
	findDur := time.Since(findStart)

	active, dormant, superseded := 0, 0, 0
	for _, rec := range recs {
		switch rec.Status {
		case imprint.StatusActive:
			active++
		case imprint.StatusDormant:
			dormant++
		default:
			superseded++
		}
	}

	fmt.Printf("wrote %d rules in %s (%d active, %d dormant, %d superseded)\n", *n, wrote, active, dormant, superseded)
	fmt.Printf("graph %d nodes %d edges in %s\n", len(g.Nodes), len(g.Edges), graphDur)
	fmt.Printf("find scope=go,naming top5 in %s → %d hits\n", findDur, len(hits))
	if len(hits) > 0 {
		fmt.Printf("  first %s score=%.3f conf=%.2f\n", hits[0].ID, hits[0].Score, hits[0].Confidence)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
