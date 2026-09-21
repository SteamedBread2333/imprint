package imprint

import (
	"errors"
	"math"
	"testing"
	"time"
)

func TestFindRecordsRecallWithoutChangingConfidence(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	v, err := Open(OpenOptions{Dir: t.TempDir(), Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	added, err := v.Add("Use camelCase identifiers", []string{"js"}, "team policy", 0.6)
	if err != nil {
		t.Fatal(err)
	}
	before, err := v.store.StatsForRules([]string{added.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Find([]string{"js"}, "camel case", 5); err != nil {
		t.Fatal(err)
	}
	rec, err := v.Get(added.ID)
	if err != nil {
		t.Fatal(err)
	}
	after, err := v.store.StatsForRules([]string{added.ID})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Confidence != 0.6 {
		t.Fatalf("confidence changed to %v", rec.Confidence)
	}
	if after[added.ID].RecallCount != 1 || after[added.ID].LastRecalledAt == nil {
		t.Fatalf("stats = %+v", after[added.ID])
	}
	if !after[added.ID].LastConfirmedAt.Equal(before[added.ID].LastConfirmedAt) {
		t.Fatalf("last_confirmed_at changed on find")
	}
}

func TestScopeAliasesApplyAcrossWritesAndFind(t *testing.T) {
	v, err := Open(OpenOptions{Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	added, err := v.Add("Use strict mode", []string{"ts", "tsx"}, "policy", 0.6)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := v.Get(added.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.Scope) != 1 || rec.Scope[0] != "typescript" {
		t.Fatalf("scope = %v", rec.Scope)
	}
	hits, err := v.Find([]string{"tsx"}, "strict", 5)
	if err != nil || len(hits) != 1 {
		t.Fatalf("hits=%+v err=%v", hits, err)
	}
}

func TestReinforceRequiresEvidenceAndUsesDiminishingGain(t *testing.T) {
	v, err := Open(OpenOptions{Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	added, err := v.Add("Write tests", []string{"go"}, "policy", 0.6)
	if err != nil {
		t.Fatal(err)
	}
	_, err = v.Reinforce(added.ID, "")
	var guarded *WriteGuardError
	if !errors.As(err, &guarded) || guarded.Code != "reinforce_evidence_required" {
		t.Fatalf("error = %#v", err)
	}
	got, err := v.Reinforce(added.ID, "user confirmed again")
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(got.Confidence-0.6875) > 1e-9 {
		t.Fatalf("confidence = %v", got.Confidence)
	}
}

func TestForgetRetainsLifecycleEvent(t *testing.T) {
	v, err := Open(OpenOptions{Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	added, err := v.Add("Temporary policy", []string{"demo"}, "policy", 0.6)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Forget(added.ID); err != nil {
		t.Fatal(err)
	}
	events, err := v.store.EventsSince(time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, event := range events {
		found = found || event.RuleID == added.ID && event.Kind == "forget"
	}
	if !found {
		t.Fatalf("events = %+v", events)
	}
}
