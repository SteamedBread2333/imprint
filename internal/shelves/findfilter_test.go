package shelves

import (
	"testing"

	"github.com/SteamedBread2333/imprint/internal/shelves/index"
)

func TestFilterFindDocuments(t *testing.T) {
	docs := []index.SearchHit{
		{ID: "a", Score: 8},
		{ID: "b", Score: 6},
		{ID: "c", Score: 4},
	}
	out := FilterFindDocuments(docs, 0.75)
	if len(out) != 2 {
		t.Fatalf("filtered = %+v", out)
	}
	if out[0].ID != "a" || out[1].ID != "b" {
		t.Fatalf("kept = %+v", out)
	}
}
