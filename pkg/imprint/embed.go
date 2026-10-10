package imprint

import "context"

// EmbedModel is the conventional embedding model id for the embed plugin.
// bge-small-zh-v1.5 emits 512-dim vectors (measured, not 384 as earlier notes
// assumed) — always read the dim from the plugin response.
const EmbedModel = "BAAI/bge-small-zh-v1.5"

// DefaultEmbedPort is the imprint-embed plugin port.
const DefaultEmbedPort = 4174

// EmbedCrossScope policies control the semantic duplicate pool.
// advisory_only scans every active+dormant rule (cross-scope hits are advisory).
// strict uses the same AND filter as find: an existing rule is in-pool only
// when it has every tag of the new claim (candidate ⊆ existing). That is
// not tag overlap — [go] vs [go,naming] and [go,style] vs [python,style]
// both miss, which keeps shared generic tags from hard-blocking.
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
// With cross-encoder/nli-deberta-v3-base, calibrate.py grid best is 0.65
// (75% duplicate recall, 0% conflict wrong-block on the sample set).
// Acceptance pair cosine ≈ 0.711 still blocks at 0.65 with relate entailment.
const DefaultEmbedDuplicateThreshold = 0.65

// DefaultRelateContradictionMin: at or above → advisory (opposite policy).
const DefaultRelateContradictionMin = 0.40

// DefaultRelateEntailmentMin: with low contradiction → duplicate block.
const DefaultRelateEntailmentMin = 0.35

// DefaultDuplicateHighCosineBlock is 0 (disabled). When > 0, cosine at or above
// this value blocks even if entailment is below RelateEntailmentMin, unless
// contradiction is at or above RelateContradictionMin (advisory). Set via yaml
// duplicate_high_cosine_block; calibrate.py sweeps it on the sample set.
const DefaultDuplicateHighCosineBlock = 0

// DefaultEmbedTimeout bounds one embedding call. On timeout or error the
// caller must degrade silently back to the lexical (Jaccard) path.
const DefaultEmbedTimeoutSeconds = 2

const maxRelatePairsPerCall = 10

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

// RelateScores are pairwise NLI-style probabilities from the embed sidecar.
type RelateScores struct {
	Contradiction float64 `json:"contradiction"`
	Entailment    float64 `json:"entailment"`
	Neutral       float64 `json:"neutral"`
}

// Relater scores claim pairs (optional embed plugin capability).
type Relater interface {
	Relate(ctx context.Context, pairs [][2]string) ([]RelateScores, error)
}
