package imprint

import (
	"sort"
	"strings"

	"github.com/SteamedBread2333/imprint/internal/privacy"
	"github.com/SteamedBread2333/imprint/internal/textseg"
)

const duplicateClaimThreshold = 0.82

type guardedText struct {
	field string
	text  string
}

func checkSensitiveText(values ...guardedText) error {
	for _, value := range values {
		if finding := privacy.CheckText(value.text); finding != nil {
			return &WriteGuardError{
				Code:    "privacy_rejected",
				Message: "write rejected because it may contain sensitive data",
				Field:   value.field,
				Kind:    finding.Kind,
			}
		}
	}
	return nil
}

func (v *Vault) checkSourcePaths(sources []DocRef) error {
	root, _ := FindProjectRootFromVault(v.Dir)
	for _, source := range sources {
		if finding := privacy.CheckSourcePath(root, source.Path); finding != nil {
			return &WriteGuardError{
				Code:    "privacy_rejected",
				Message: "source path is not allowed",
				Field:   "sources.path",
				Kind:    finding.Kind,
			}
		}
	}
	return nil
}

func (v *Vault) duplicateCandidates(claim string, scope []string) ([]DuplicateCandidate, error) {
	want := tokenSet(textseg.Tokenize(claim))
	if len(want) == 0 {
		return nil, nil
	}
	records, err := v.store.ActiveCandidates(nil, 0)
	if err != nil {
		return nil, err
	}
	var out []DuplicateCandidate
	for _, rec := range records {
		if !scopesOverlap(scope, rec.Scope) {
			continue
		}
		got := tokenSet(textseg.Tokenize(rec.Claim))
		score := 0.0
		if strings.EqualFold(strings.TrimSpace(claim), strings.TrimSpace(rec.Claim)) {
			score = 1
		} else {
			// Very short token sets are dominated by dropped one-letter terms
			// and generic verbs, which makes Jaccard too eager.
			if len(want) < 3 || len(got) < 3 {
				continue
			}
			score = jaccard(want, got)
		}
		if score >= duplicateClaimThreshold {
			out = append(out, DuplicateCandidate{ID: rec.ID, Score: score, Claim: rec.Claim})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].ID < out[j].ID
		}
		return out[i].Score > out[j].Score
	})
	if len(out) > 3 {
		out = out[:3]
	}
	return out, nil
}

func tokenSet(tokens []string) map[string]struct{} {
	out := make(map[string]struct{}, len(tokens))
	for _, token := range tokens {
		out[token] = struct{}{}
	}
	return out
}

func jaccard(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	intersection := 0
	for token := range a {
		if _, ok := b[token]; ok {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	return float64(intersection) / float64(union)
}

func scopesOverlap(a, b []string) bool {
	seen := make(map[string]struct{}, len(a))
	for _, scope := range a {
		seen[strings.ToLower(scope)] = struct{}{}
	}
	for _, scope := range b {
		if _, ok := seen[strings.ToLower(scope)]; ok {
			return true
		}
	}
	return false
}
