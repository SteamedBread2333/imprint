package shelves_test

import (
	"testing"

	"github.com/SteamedBread2333/imprint/internal/shelves"
)

func TestMatchFindQuery(t *testing.T) {
	on := shelves.Config{Enabled: true}
	off := shelves.Config{Enabled: false}
	if !shelves.MatchFindQuery(on, "storage", "") {
		t.Fatal("expected query match")
	}
	if !shelves.MatchFindQuery(on, "", "否定式堆砌") {
		t.Fatal("expected query_local match")
	}
	if !shelves.MatchFindQuery(on, "切忌否定式堆砌", "") {
		t.Fatal("expected CJK query match")
	}
	if shelves.MatchFindQuery(on, "", "") {
		t.Fatal("expected empty miss")
	}
	if shelves.MatchFindQuery(off, "storage", "否定式堆砌") {
		t.Fatal("expected disabled miss")
	}
}
