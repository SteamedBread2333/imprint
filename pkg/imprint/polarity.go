package imprint

import (
	"regexp"
	"strings"
)

// Polarity guard: a cheap lexical signal that catches antonym conflicts.
//
// Calibration on BAAI/bge-small-zh-v1.5 (imprint-embed-sidecar/calibrate.py,
// 20 pairs per group) showed cosine alone cannot separate duplicates from
// conflicts:
//
//	duplicate  median 0.823  (min 0.652)
//	conflict   median 0.753  (max 1.000 — "Prefer tabs over spaces" vs
//	                          "Prefer spaces over tabs" is a perfect cosine)
//	unrelated  median 0.528
//
// Cosine measures topic relatedness, and antonyms are maximally topically
// related. The guard is therefore the second signal: high cosine plus a
// polarity asymmetry means "same topic, likely opposite policy", which must
// not be hard-rejected — the LLM decides.
//
// Calibration principle: bias wide on the antonym table and rely on
// advisory (not reject) for the consequences. A false positive only costs
// the user one extra click to confirm "no, this is actually different" — a
// missed polarity signal costs a hard rejection that wedges two opposite
// policies into the user's vault. The current add/write vs skip/remove pair
// is intentionally broad: in normal English those tokens rarely co-occur
// outside rule-style sentences, but in imprint's corpus they always mean
// opposite intent.
//
// Keep this in sync with imprint-embed-sidecar/polarity.py.

// negationMarkers are explicit polarity words. "no " / "not " keep their
// trailing space to avoid matching inside words like "notify" / "notation".
// The bare single character "别" is deliberately absent: substring matching
// would also fire inside 分别 / 别人 / 特别 / 性别, none of which negate.
// Only multi-character imperative forms ("别用", "别写", ...) are listed.
var negationMarkers = []string{
	"never", "do not", "don't", "dont", "avoid", "without", "instead of",
	"rather than", "skip", "ignore", "forbid", "no ", "not ", "bare ",
	"must not", "should not", "refrain",
	"不要", "禁止", "避免", "不可", "不能", "而非", "而不是", "无需", "禁用",
	"别用", "别写", "别加", "别删", "别改", "别弄", "别碰", "别提交",
}

// antonymPairs are token pairs where one side in each claim signals an
// opposite policy (tabs vs spaces, wrap vs bare, ...).
var antonymPairs = [][2][]string{
	{{"tabs", "tab"}, {"spaces", "space"}},
	{{"snake_case", "snakecase"}, {"camelcase", "camel_case"}},
	{{"short", "shorter", "small"}, {"long", "longer", "large"}},
	{{"wrap", "wrapping", "wrapped"}, {"bare", "unwrap", "raw"}},
	{{"structured"}, {"plain", "formatted"}},
	{{"composition"}, {"inheritance"}},
	{{"vendor", "vendored"}, {"novendor", "no_vendor"}},
	{{"early return", "early returns"}, {"single exit", "one exit"}},
	{{"add", "write", "include"}, {"skip", "omit", "remove"}},
	{{"explicit", "explicitly"}, {"ignore", "discard"}},
	{{"pin", "pinned", "fixed"}, {"floating", "range", "latest"}},
	{{"english"}, {"chinese"}},
}

var wordRe = regexp.MustCompile(`[a-z0-9_]+`)

func polarityTokens(text string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, w := range wordRe.FindAllString(strings.ToLower(text), -1) {
		out[w] = struct{}{}
	}
	return out
}

func hasNegation(text string) bool {
	lowered := strings.ToLower(text)
	for _, marker := range negationMarkers {
		if strings.Contains(lowered, marker) {
			return true
		}
	}
	return false
}

// hasAntonymPair reports whether the two claims sit on opposite sides of a
// known antonym pair (tabs vs spaces, wrap vs bare, ...).
//
// Word-reordering with identical token sets ("Use pnpm workspaces always"
// vs "Always use pnpm workspaces") is intentionally NOT treated as a
// polarity conflict: the rules express the same intent and Jaccard's hard
// reject is the correct behaviour. Treating such pairs as advisory would
// silently let duplicates slip past the lexical gate.
func hasAntonymPair(a, b string) bool {
	ta, tb := polarityTokens(a), polarityTokens(b)
	for _, pair := range antonymPairs {
		left, right := pair[0], pair[1]
		if intersects(ta, left) && intersects(tb, right) {
			return true
		}
		if intersects(tb, left) && intersects(ta, right) {
			return true
		}
	}
	return false
}

func intersects(tokens map[string]struct{}, group []string) bool {
	for _, g := range group {
		if _, ok := tokens[g]; ok {
			return true
		}
	}
	return false
}

// PolarityConflict reports whether two claims likely express opposite
// policies. Measured on the calibration set: fires on 18/20 antonym pairs and
// cuts the conflict false-positive rate of the semantic duplicate gate from
// 70% to 10% at the chosen threshold.
func PolarityConflict(a, b string) bool {
	if hasAntonymPair(a, b) {
		return true
	}
	return hasNegation(a) != hasNegation(b)
}
