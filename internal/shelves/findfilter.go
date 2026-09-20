package shelves

import "github.com/SteamedBread2333/imprint/internal/shelves/index"

// FindDocMinRelativeScore drops weak document hits far below the top BM25 score.
const FindDocMinRelativeScore = 0.75

// FilterFindDocuments keeps hits within minRelative of the best score (0–1).
func FilterFindDocuments(docs []index.SearchHit, minRelative float64) []index.SearchHit {
	if len(docs) == 0 || minRelative <= 0 {
		return docs
	}
	top := docs[0].Score
	for _, d := range docs[1:] {
		if d.Score > top {
			top = d.Score
		}
	}
	if top <= 0 {
		return docs
	}
	cutoff := top * minRelative
	out := make([]index.SearchHit, 0, len(docs))
	for _, d := range docs {
		if d.Score >= cutoff {
			out = append(out, d)
		}
	}
	return out
}
