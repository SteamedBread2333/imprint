package imprint

import "context"

// EmbedModel is the conventional embedding model id for the embed plugin.
// bge-small-zh-v1.5 emits 512-dim vectors (measured, not 384 as earlier notes
// assumed) — always read the dim from the plugin response.
const EmbedModel = "BAAI/bge-small-zh-v1.5"

// DefaultEmbedPort is the imprint-embed plugin port.
const DefaultEmbedPort = 4174

// EmbedCrossScope policies control whether the semantic duplicate gate runs
// across all rules (advisory_only) or only across rules whose scope
// overlaps the candidate (strict).
const (
	EmbedCrossScopeAdvisoryOnly = "advisory_only"
	EmbedCrossScopeStrict       = "strict"
)

// DefaultEmbedDuplicateThreshold is the semantic duplicate cosine threshold,
// calibrated by imprint-embed-sidecar/calibrate.py on 20 pairs per group with
// bge-small-zh-v1.5:
//
//	duplicate  median 0.823 (min 0.652)
//	conflict   median 0.753 (max 1.000)
//	unrelated  median 0.528
//
// 0.70 is the highest value that still catches the acceptance pair
// ("Go exported identifiers must use PascalCase" vs
// "Exported things use Pascal Case naming", cosine 0.711) while keeping
// unrelated false positives at 0%. Conflict pairs are additionally filtered by
// PolarityConflict, which drops their false-positive rate from 70% to 10% here.
// Do not raise this without re-running the calibration.
const DefaultEmbedDuplicateThreshold = 0.70

// DefaultEmbedTimeout bounds one embedding call. On timeout or error the
// caller must degrade silently back to the lexical (Jaccard) path.
const DefaultEmbedTimeoutSeconds = 2

// Embedder produces dense vectors for semantic duplicate detection.
// Implementations must be safe for concurrent use; failures must surface as
// errors so callers can degrade to the lexical path without blocking writes.
type Embedder interface {
	// Embed returns one vector per text, in order.
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	// Model identifies the vector space; vectors from different models are
	// not comparable and are ignored by consumers.
	Model() string
}
