package imprint

import (
	"fmt"
	"testing"
	"time"
)

func BenchmarkFind3000Rules(b *testing.B) {
	dir := b.TempDir()
	v, err := Open(OpenOptions{Dir: dir, Now: func() time.Time {
		return time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	}})
	if err != nil {
		b.Fatal(err)
	}
	recs := make([]*Record, 3000)
	for i := range recs {
		recs[i] = &Record{
			ID:          fmt.Sprintf("r-2026-09-11-%03d", i+1),
			Claim:       fmt.Sprintf("Go exported names use PascalCase rule %d", i),
			Scope:       []string{"go", "naming"},
			Confidence:  0.7,
			Status:      StatusActive,
			CreatedAt:   time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC),
			UpdatedAt:   time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC),
			EvidenceLog: []Evidence{{At: time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC), Kind: EvidenceOriginal, Text: "PascalCase"}},
		}
	}
	if err := v.ImportRecords(recs); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := v.Find([]string{"go", "naming"}, "PascalCase", 5); err != nil {
			b.Fatal(err)
		}
	}
}
