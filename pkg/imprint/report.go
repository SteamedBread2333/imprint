package imprint

import (
	"sort"
	"time"

	"github.com/SteamedBread2333/imprint/internal/textseg"
)

// Report summarizes lifecycle events and rule maintenance signals.
func (v *Vault) Report(days int) (*ReportResult, error) {
	if days <= 0 {
		days = 30
	}
	since := v.instant().Add(-time.Duration(days) * 24 * time.Hour)
	events, err := v.store.EventsSince(since)
	if err != nil {
		return nil, err
	}
	recs, err := v.loadAll()
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(recs))
	for _, rec := range recs {
		ids = append(ids, rec.ID)
	}
	stats, err := v.store.StatsForRules(ids)
	if err != nil {
		return nil, err
	}
	out := &ReportResult{
		Since:       since,
		EventCounts: map[string]int{},
		Rules:       make([]ReportRule, 0, len(recs)),
	}
	for _, event := range events {
		out.EventCounts[event.Kind]++
	}
	var activeIDs []string
	for _, rec := range recs {
		st := stats[rec.ID]
		row := ReportRule{
			ID: rec.ID, Claim: rec.Claim, Status: string(rec.Status),
			Confidence: rec.Confidence, RecallCount: st.RecallCount,
			LastRecalledAt: st.LastRecalledAt, LastConfirmedAt: st.LastConfirmedAt,
		}
		switch {
		case rec.Status == StatusDormant:
			row.Recommendation = "confirm and reinforce, or forget"
		case rec.Status == StatusActive && st.RecallCount == 0 && rec.CreatedAt.Before(since):
			row.Recommendation = "zero recall; review or forget"
		case rec.Status == StatusActive && rec.Confidence < 0.5:
			row.Recommendation = "low confidence; review"
		}
		out.Rules = append(out.Rules, row)
		if rec.Status == StatusActive {
			activeIDs = append(activeIDs, rec.ID)
		}
	}
	for i, a := range recs {
		if a.Status != StatusActive {
			continue
		}
		for _, b := range recs[i+1:] {
			if b.Status != StatusActive || !scopesOverlap(a.Scope, b.Scope) {
				continue
			}
			score := jaccard(tokenSet(textseg.Tokenize(a.Claim)), tokenSet(textseg.Tokenize(b.Claim)))
			if score >= duplicateClaimThreshold {
				out.Duplicates = append(out.Duplicates, DuplicatePair{A: a.ID, B: b.ID, Score: score})
			}
		}
	}
	out.Conflicts, err = v.ConflictsAmong(activeIDs)
	if err != nil {
		return nil, err
	}
	sort.Slice(out.Rules, func(i, j int) bool { return out.Rules[i].ID < out.Rules[j].ID })
	return out, nil
}
