package imprint

import (
	"fmt"
	"strings"
	"time"
)

// Reinforce applies diminishing confidence gain after explicit reaffirmation.
func (v *Vault) Reinforce(id, evidence string) (*ReinforceResult, error) {
	return v.ReinforceQueryLocal(id, evidence, "")
}

// ReinforceQueryLocal is Reinforce; when queryLocal is non-empty it updates stored query_local.
func (v *Vault) ReinforceQueryLocal(id, evidence, queryLocal string) (*ReinforceResult, error) {
	start := time.Now()
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}
	if strings.TrimSpace(evidence) == "" {
		v.emit(TelemetryEvent{Op: "write_rejected", Code: "reinforce_evidence_required"})
		return nil, &WriteGuardError{
			Code:    "reinforce_evidence_required",
			Message: "reinforce requires non-empty user reaffirmation evidence",
			Field:   "evidence",
		}
	}
	if err := checkSensitiveText(
		guardedText{field: "evidence", text: evidence},
		guardedText{field: "query_local", text: queryLocal},
	); err != nil {
		v.emit(TelemetryEvent{Op: "write_rejected", Code: "privacy_rejected"})
		return nil, err
	}
	now := v.instant()
	ql := MergeQueryLocalForStore(queryLocal, evidence)
	confidence, count, err := v.store.ReinforceRecord(id, now, Evidence{
		At:   now,
		Kind: EvidenceReinforce,
		Text: evidence,
	}, ql, MaxConfidence)
	if err != nil {
		return nil, err
	}
	v.emit(TelemetryEvent{Op: "reinforce", LatencyMS: time.Since(start).Milliseconds()})
	return &ReinforceResult{
		ID:                 id,
		Confidence:         confidence,
		ReinforcementCount: count,
	}, nil
}

// Supersede archives oldID as superseded and writes a new active rule that points at it.
func (v *Vault) Supersede(oldID, newClaim string, newScope []string, reason, originalText string) (*SupersedeResult, error) {
	return v.SupersedeWithSources(oldID, newClaim, newScope, reason, originalText, nil, "")
}

// SupersedeWithSources archives oldID and writes a new rule, optionally replacing inherited sources.
func (v *Vault) SupersedeWithSources(oldID, newClaim string, newScope []string, reason, originalText string, sources []DocRef, queryLocal string) (*SupersedeResult, error) {
	start := time.Now()
	oldID = strings.TrimSpace(oldID)
	newClaim = strings.TrimSpace(newClaim)
	if oldID == "" {
		return nil, fmt.Errorf("old_id is required")
	}
	if newClaim == "" {
		return nil, fmt.Errorf("new_claim is required")
	}
	newScope = v.cleanScope(newScope)
	if len(newScope) == 0 {
		return nil, fmt.Errorf("new_scope is required")
	}
	if err := checkSensitiveText(
		guardedText{field: "claim", text: newClaim},
		guardedText{field: "reason", text: reason},
		guardedText{field: "text", text: originalText},
		guardedText{field: "query_local", text: queryLocal},
	); err != nil {
		v.emit(TelemetryEvent{Op: "write_rejected", Code: "privacy_rejected"})
		return nil, err
	}
	if err := v.checkSourcePaths(sources); err != nil {
		v.emit(TelemetryEvent{Op: "write_rejected", Code: "privacy_rejected"})
		return nil, err
	}
	old, err := v.load(oldID)
	if err != nil {
		return nil, err
	}
	expectedUpdatedAt := old.UpdatedAt
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
	if err := v.store.SupersedePair(old, newRec, expectedUpdatedAt); err != nil {
		return nil, err
	}
	v.emit(TelemetryEvent{Op: "supersede", ScopeCount: len(newScope), LatencyMS: time.Since(start).Milliseconds()})
	return &SupersedeResult{ID: newID, SupersededOldID: old.ID}, nil
}

// Sweep decays untouched rules and archives those below the dormant threshold.
func (v *Vault) Sweep(decayDays int, decayAmount, dormantThreshold float64) (*SweepResult, error) {
	start := time.Now()
	if decayDays <= 0 {
		decayDays = DefaultDecayDays
	}
	if decayAmount <= 0 {
		decayAmount = DefaultDecayAmount
	}
	if dormantThreshold <= 0 {
		dormantThreshold = DefaultDormantThresh
	}
	now := v.instant()
	cutoff := now.Add(-time.Duration(decayDays) * 24 * time.Hour)
	decayed, archived, err := v.store.SweepActive(cutoff, now, decayAmount, dormantThreshold)
	if err != nil {
		return nil, err
	}
	v.emit(TelemetryEvent{Op: "sweep", LatencyMS: time.Since(start).Milliseconds()})
	return &SweepResult{Decayed: decayed, Archived: archived}, nil
}
