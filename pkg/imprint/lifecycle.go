package imprint

import (
	"fmt"
	"strings"
	"time"
)

func (v *Vault) touch(r *Record, now time.Time) {
	r.UpdatedAt = now
	r.LastTouchedAt = now
}

// Reinforce raises confidence by 0.1 (capped at 0.95) and increments reinforcement_count.
func (v *Vault) Reinforce(id, evidence string) (*ReinforceResult, error) {
	return v.ReinforceQueryLocal(id, evidence, "")
}

// ReinforceQueryLocal is Reinforce; when queryLocal is non-empty it updates stored query_local.
func (v *Vault) ReinforceQueryLocal(id, evidence, queryLocal string) (*ReinforceResult, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}
	rec, err := v.load(id)
	if err != nil {
		return nil, err
	}
	if rec.Status == StatusSuperseded {
		return nil, fmt.Errorf("record %s is superseded and cannot be reinforced", id)
	}
	now := v.instant()
	rec.Confidence = clampConfidence(rec.Confidence + 0.1)
	rec.ReinforcementCount++
	v.touch(rec, now)
	if rec.Status == StatusDormant {
		rec.Status = StatusActive
	}
	if ql := MergeQueryLocalForStore(queryLocal, evidence); ql != "" {
		rec.QueryLocal = ql
	}
	rec.EvidenceLog = append(rec.EvidenceLog, Evidence{
		At:   now,
		Kind: EvidenceReinforce,
		Text: evidence,
	})
	if err := v.save(rec); err != nil {
		return nil, err
	}
	return &ReinforceResult{
		ID:                 rec.ID,
		Confidence:         rec.Confidence,
		ReinforcementCount: rec.ReinforcementCount,
	}, nil
}

// Supersede archives oldID as superseded and writes a new active rule that points at it.
func (v *Vault) Supersede(oldID, newClaim string, newScope []string, reason, originalText string) (*SupersedeResult, error) {
	return v.SupersedeWithSources(oldID, newClaim, newScope, reason, originalText, nil, "")
}

// SupersedeWithSources archives oldID and writes a new rule, optionally replacing inherited sources.
func (v *Vault) SupersedeWithSources(oldID, newClaim string, newScope []string, reason, originalText string, sources []DocRef, queryLocal string) (*SupersedeResult, error) {
	oldID = strings.TrimSpace(oldID)
	newClaim = strings.TrimSpace(newClaim)
	if oldID == "" {
		return nil, fmt.Errorf("old_id is required")
	}
	if newClaim == "" {
		return nil, fmt.Errorf("new_claim is required")
	}
	newScope = cleanScope(newScope)
	if len(newScope) == 0 {
		return nil, fmt.Errorf("new_scope is required")
	}
	old, err := v.load(oldID)
	if err != nil {
		return nil, err
	}
	now := v.instant()
	text := originalText
	if strings.TrimSpace(text) == "" {
		text = reason
	}
	useSources := old.Sources
	if len(sources) > 0 {
		useSources = cleanDocRefs(sources)
	}
	useQueryLocal := old.QueryLocal
	if ql := strings.TrimSpace(queryLocal); ql != "" {
		useQueryLocal = ql
	}
	newID, err := v.store.NextID(now)
	if err != nil {
		return nil, err
	}
	newRec := &Record{
		ID:                 newID,
		Claim:              newClaim,
		Scope:              newScope,
		Confidence:         old.Confidence,
		Status:             StatusActive,
		ReinforcementCount: 0,
		CreatedAt:          now,
		UpdatedAt:          now,
		LastTouchedAt:      now,
		Supersedes:         []string{old.ID},
		Related:            []string{old.ID},
		Sources:            useSources,
		QueryLocal:         MergeQueryLocalForStore(useQueryLocal, text),
		EvidenceLog:        []Evidence{{At: now, Kind: EvidenceOriginal, Text: text}},
		Path:               old.Path,
	}
	if reason != "" {
		newRec.EvidenceLog = append(newRec.EvidenceLog, Evidence{
			At: now, Kind: EvidenceSupersede, Text: reason,
		})
	}
	old.Status = StatusSuperseded
	old.UpdatedAt = now
	note := "superseded by " + newID
	if reason != "" {
		note += ": " + reason
	}
	old.EvidenceLog = append(old.EvidenceLog, Evidence{
		At: now, Kind: EvidenceSupersede, Text: note,
	})
	if err := v.store.SupersedePair(old, newRec); err != nil {
		return nil, err
	}
	return &SupersedeResult{ID: newID, SupersededOldID: old.ID}, nil
}

// Sweep decays untouched rules and archives those below the dormant threshold.
func (v *Vault) Sweep(decayDays int, decayAmount, dormantThreshold float64) (*SweepResult, error) {
	if decayDays <= 0 {
		decayDays = DefaultDecayDays
	}
	if decayAmount <= 0 {
		decayAmount = DefaultDecayAmount
	}
	if dormantThreshold <= 0 {
		dormantThreshold = DefaultDormantThresh
	}
	recs, err := v.store.AllRecords(false)
	if err != nil {
		return nil, err
	}
	now := v.instant()
	cutoff := now.Add(-time.Duration(decayDays) * 24 * time.Hour)
	result := &SweepResult{}
	for _, rec := range recs {
		if rec.Status != StatusActive {
			continue
		}
		changed := false
		if !rec.LastTouchedAt.After(cutoff) {
			rec.Confidence -= decayAmount
			if rec.Confidence < 0 {
				rec.Confidence = 0
			}
			rec.UpdatedAt = now
			result.Decayed++
			changed = true
		}
		if rec.Confidence < dormantThreshold {
			rec.Status = StatusDormant
			rec.UpdatedAt = now
			if err := v.save(rec); err != nil {
				return nil, err
			}
			result.Archived++
			continue
		}
		if changed {
			if err := v.save(rec); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}
