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
	rec.EvidenceLog = append(rec.EvidenceLog, Evidence{
		At:   now,
		Kind: EvidenceReinforce,
		Text: evidence,
	})
	archived := rec.Status != StatusActive
	if err := v.save(rec, archived); err != nil {
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
	sources := old.Sources
	if len(sources) == 0 {
		sources = nil
	}
	added, err := v.AddRecord(newClaim, newScope, text, old.Confidence, []string{old.ID}, []string{old.ID}, nil, sources)
	if err != nil {
		return nil, err
	}
	if reason != "" || originalText != "" {
		fresh, err := v.load(added.ID)
		if err != nil {
			return nil, err
		}
		if reason != "" {
			fresh.EvidenceLog = append(fresh.EvidenceLog, Evidence{
				At:   now,
				Kind: EvidenceSupersede,
				Text: reason,
			})
			if err := v.save(fresh, false); err != nil {
				return nil, err
			}
		}
	}
	old.Status = StatusSuperseded
	old.UpdatedAt = now
	note := "superseded by " + added.ID
	if reason != "" {
		note += ": " + reason
	}
	old.EvidenceLog = append(old.EvidenceLog, Evidence{
		At:   now,
		Kind: EvidenceSupersede,
		Text: note,
	})
	if err := v.save(old, true); err != nil {
		return nil, err
	}
	return &SupersedeResult{ID: added.ID, SupersededOldID: old.ID}, nil
}

// SupersedeWithSources archives oldID and writes a new rule, optionally replacing inherited sources.
func (v *Vault) SupersedeWithSources(oldID, newClaim string, newScope []string, reason, originalText string, sources []DocRef) (*SupersedeResult, error) {
	if len(sources) == 0 {
		return v.Supersede(oldID, newClaim, newScope, reason, originalText)
	}
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
	added, err := v.AddRecord(newClaim, newScope, text, old.Confidence, []string{old.ID}, []string{old.ID}, nil, sources)
	if err != nil {
		return nil, err
	}
	if reason != "" || originalText != "" {
		fresh, err := v.load(added.ID)
		if err != nil {
			return nil, err
		}
		if reason != "" {
			fresh.EvidenceLog = append(fresh.EvidenceLog, Evidence{
				At:   now,
				Kind: EvidenceSupersede,
				Text: reason,
			})
			if err := v.save(fresh, false); err != nil {
				return nil, err
			}
		}
	}
	old.Status = StatusSuperseded
	old.UpdatedAt = now
	note := "superseded by " + added.ID
	if reason != "" {
		note += ": " + reason
	}
	old.EvidenceLog = append(old.EvidenceLog, Evidence{
		At:   now,
		Kind: EvidenceSupersede,
		Text: note,
	})
	if err := v.save(old, true); err != nil {
		return nil, err
	}
	return &SupersedeResult{ID: added.ID, SupersededOldID: old.ID}, nil
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
	recs, err := v.loadAll()
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
			if err := v.save(rec, true); err != nil {
				return nil, err
			}
			result.Archived++
			continue
		}
		if changed {
			if err := v.save(rec, false); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}
