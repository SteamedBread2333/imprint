package plugin

import (
	"context"
	"testing"
)

// StartEnabledExcept must call startOne for every enabled plugin whose id is
// not in the omit list, and skip the rest. We do not want to fork real
// processes in a unit test, so we exploit the early-out that startOne
// applies on an empty Package path: an entry with no Package cannot be
// started, but it can still be iterated, and the omit filter is checked
// before that check.
//
// Concretely: with two enabled entries ("embed" omitted and "shelves"
// included) and both having no Package, StartEnabledExcept must report
// both as Error in the resulting Status list (because startOne failed)
// but record zero procs for "embed" — i.e. the omit path was taken.
func TestStartEnabledExceptSkipsOmittedPlugin(t *testing.T) {
	cfg := &Config{
		Plugins: map[string]PluginEntry{
			"embed":  {Enabled: true, Package: ""},
			"shelves": {Enabled: true, Package: ""},
		},
	}
	mgr := NewManager(cfg)
	statuses, err := mgr.StartEnabledExcept(context.Background(), []string{"embed"})
	if err != nil {
		t.Fatalf("StartEnabledExcept returned err: %v", err)
	}
	var embed, shelves *Status
	for i := range statuses {
		s := statuses[i]
		switch s.ID {
		case "embed":
			embed = &s
		case "shelves":
			shelves = &s
		}
	}
	if embed == nil {
		t.Fatal("embed status missing from result")
	}
	if shelves == nil {
		t.Fatal("shelves status missing from result")
	}
	// The decisive check: when omitted, embed must not have been pushed
	// into m.status. When included (and startOne fails for other reasons
	// like an empty Package), shelves must have been pushed.
	mgr.mu.Lock()
	_, embedInStatus := mgr.status["embed"]
	_, shelvesInStatus := mgr.status["shelves"]
	mgr.mu.Unlock()
	if embedInStatus {
		t.Fatal("embed must not appear in mgr.status when omitted")
	}
	if !shelvesInStatus {
		t.Fatal("shelves must appear in mgr.status (startOne was called)")
	}
}

// Disabled plugins must be skipped regardless of the omit list.
func TestStartEnabledExceptRespectsDisabledFlag(t *testing.T) {
	cfg := &Config{
		Plugins: map[string]PluginEntry{
			"embed": {Enabled: false, Package: ""},
		},
	}
	mgr := NewManager(cfg)
	statuses, err := mgr.StartEnabledExcept(context.Background(), nil)
	if err != nil {
		t.Fatalf("StartEnabledExcept returned err: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	st := statuses[0]
	if st.ID != "embed" {
		t.Fatalf("status ID = %q, want embed", st.ID)
	}
	if st.Error != "" {
		t.Fatalf("disabled plugin must not be started, got error: %q", st.Error)
	}
	if st.Running {
		t.Fatal("disabled plugin reported as running")
	}
}