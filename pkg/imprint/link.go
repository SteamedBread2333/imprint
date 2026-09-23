package imprint

import (
	"fmt"
	"strings"
	"time"
)

// Link adds relationship edges from an existing rule to other existing rules.
// It is the post-hoc companion of the add-time conflicts field: when add
// returns advisory Similar candidates (same topic, opposite polarity), the
// caller confirms with the user and then records the opposition here so
// future find results penalize the weaker side of the pair.
//
// Edges are directional; conflicts_with is consumed symmetrically by
// penalizeConflicts, so one direction is enough. Inserts are idempotent.
func (v *Vault) Link(id string, related, conflicts []string) (*LinkResult, error) {
	start := time.Now()
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}
	related = cleanStrings(related)
	conflicts = cleanStrings(conflicts)
	if len(related) == 0 && len(conflicts) == 0 {
		return nil, fmt.Errorf("related or conflicts is required")
	}
	if _, err := v.load(id); err != nil {
		return nil, err
	}
	res := &LinkResult{ID: id}
	if len(related) > 0 {
		n, err := v.store.AddEdges(id, related, "related")
		if err != nil {
			return nil, err
		}
		res.RelatedAdded = n
	}
	if len(conflicts) > 0 {
		n, err := v.store.AddEdges(id, conflicts, "conflicts_with")
		if err != nil {
			return nil, err
		}
		res.ConflictsAdded = n
	}
	v.emit(TelemetryEvent{Op: "link", RuleHitCount: res.RelatedAdded + res.ConflictsAdded, LatencyMS: time.Since(start).Milliseconds()}, newTraceID())
	return res, nil
}
