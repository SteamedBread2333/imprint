package imprint

import (
	"testing"
	"time"
)

func TestFindReturnsDormantAsLowWeightFallbackWithoutWaking(t *testing.T) {
	dir := t.TempDir()
	oldTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	v, err := OpenWithNow(dir, func() time.Time { return oldTime })
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	added, err := v.Add("Use camelCase for dormant variables", []string{"js", "naming"}, "old policy", 0.32)
	if err != nil {
		t.Fatal(err)
	}
	v.now = func() time.Time { return oldTime.AddDate(0, 0, 100) }
	if _, err := v.Sweep(90, 0.05, 0.3); err != nil {
		t.Fatal(err)
	}

	hits, err := v.Find([]string{"js", "naming"}, "Camel Case dormant variables", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].ID != added.ID {
		t.Fatalf("dormant fallback hits = %+v", hits)
	}
	if hits[0].Status != string(StatusDormant) || !hits[0].WakeCandidate {
		t.Fatalf("dormant hit metadata = %+v", hits[0])
	}
	got, err := v.Get(added.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusDormant {
		t.Fatalf("find woke dormant rule: status=%s", got.Status)
	}
}

func TestReinforceWakesDormantRule(t *testing.T) {
	dir := t.TempDir()
	oldTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	v, err := OpenWithNow(dir, func() time.Time { return oldTime })
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	added, err := v.Add("Keep the dormant policy", []string{"general"}, "old", 0.2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Sweep(90, 0.05, 0.3); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Reinforce(added.ID, "user explicitly confirmed it"); err != nil {
		t.Fatal(err)
	}
	got, err := v.Get(added.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusActive {
		t.Fatalf("reinforce status=%s, want active", got.Status)
	}
}
