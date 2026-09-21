package imprint

import (
	"math"
	"testing"
	"time"
)

func TestInheritConfidence(t *testing.T) {
	tests := []struct {
		old, baseline, alpha, want float64
	}{
		{0.6875, 0.6, 0.20, 0.67},
		{0.8, 0.6, 0, 0.8},
		{0.8, 0.6, 1, 0.6},
		{0.5, 0.6, 0.20, 0.6},
		{0.6, 0.6, 0.20, 0.6},
		{0.8, 0.6, 1.5, 0.6},
		{0.8, 0.6, -0.1, 0.8},
	}
	for _, tc := range tests {
		got := inheritConfidence(tc.old, tc.baseline, tc.alpha)
		if math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("inheritConfidence(%v, %v, %v) = %v want %v", tc.old, tc.baseline, tc.alpha, got, tc.want)
		}
	}
}

func TestSupersedeTrustGapDecay(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	zero, one := 0.0, 1.0
	cases := []struct {
		name  string
		alpha *float64
		want  float64
	}{
		{"default", nil, 0.68},
		{"copy", &zero, 0.7},
		{"baseline", &one, 0.6},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v, err := Open(OpenOptions{
				Dir:              t.TempDir(),
				Now:              func() time.Time { return now },
				InheritanceAlpha: tc.alpha,
			})
			if err != nil {
				t.Fatal(err)
			}
			defer v.Close()
			old, err := v.Add("Old policy", []string{"go"}, "old", 0.7)
			if err != nil {
				t.Fatal(err)
			}
			res, err := v.Supersede(old.ID, "New policy", []string{"go"}, "changed", "new wording")
			if err != nil {
				t.Fatal(err)
			}
			got, err := v.Get(res.ID)
			if err != nil {
				t.Fatal(err)
			}
			if math.Abs(got.Confidence-tc.want) > 1e-9 {
				t.Fatalf("confidence = %v want %v", got.Confidence, tc.want)
			}
		})
	}
}
