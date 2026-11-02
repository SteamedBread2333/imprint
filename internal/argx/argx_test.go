package argx

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSplitCSV(t *testing.T) {
	cases := map[string][]string{
		"":               nil,
		"  ":             nil,
		"go":             {"go"},
		" go , ,naming,": {"go", "naming"},
	}
	for in, want := range cases {
		if got := SplitCSV(in); !reflect.DeepEqual(got, want) {
			t.Errorf("SplitCSV(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestParseSince(t *testing.T) {
	if got, err := ParseSince("", "since"); err != nil || !got.IsZero() {
		t.Fatalf("empty: %v %v", got, err)
	}
	got, err := ParseSince("2026-09-11", "since")
	if err != nil || !got.Equal(time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("date: %v %v", got, err)
	}
	if _, err := ParseSince("2026-09-11T08:00:00+08:00", "since"); err != nil {
		t.Fatalf("rfc3339: %v", err)
	}
	_, err = ParseSince("yesterday", "--since")
	if err == nil || !strings.Contains(err.Error(), "--since") {
		t.Fatalf("want labeled error, got %v", err)
	}
}
