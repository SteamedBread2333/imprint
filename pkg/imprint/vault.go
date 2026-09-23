package imprint

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/SteamedBread2333/imprint/internal/vault/sqlite"
)

// Vault is a SQLite-backed imprint store at Dir/vault.db.
type Vault struct {
	Dir          string
	store        *sqlite.Store
	now          func() time.Time
	telemetry    EventSink
	inheritAlpha float64
	embed        Embedder
	embedTimeout time.Duration
	embedThresh  float64
}

// OpenOptions configures a vault. Construction is immutable after Open.
type OpenOptions struct {
	Dir       string
	Now       func() time.Time
	Telemetry EventSink
	// InheritanceAlpha is supersede trust-gap decay (0=copy old, 1=baseline).
	// nil uses DefaultInheritanceAlpha (0.20).
	InheritanceAlpha *float64
	// Embed enables the semantic duplicate path when non-nil. All failures
	// degrade silently to the lexical Jaccard path.
	Embed Embedder
	// EmbedDuplicateThreshold is the semantic-duplicate cosine threshold.
	// <= 0 uses DefaultEmbedDuplicateThreshold.
	EmbedDuplicateThreshold float64
	// EmbedTimeout bounds each embedding call. <= 0 uses DefaultEmbedTimeoutSeconds.
	EmbedTimeout time.Duration
}

// TelemetryEvent contains content-free operational measurements.
type TelemetryEvent struct {
	At             time.Time
	Op             string
	ScopeCount     int
	QueryTermCount int
	RuleHitCount   int
	DocHitCount    int
	WakeCandidate  bool
	LatencyMS      int64
	Code           string
	// TraceID ties every event of one logical operation (e.g. an add that
	// embeds, then rejects) together; empty for events without a trace.
	TraceID string
}

// EventSink receives privacy-safe telemetry.
type EventSink interface {
	Record(TelemetryEvent) error
}

// Open creates (if needed) and returns a configured vault.
func Open(opts OpenOptions) (*Vault, error) {
	if strings.TrimSpace(opts.Dir) == "" {
		return nil, fmt.Errorf("vault path is empty")
	}
	abs, err := filepath.Abs(opts.Dir)
	if err != nil {
		return nil, err
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	alpha := DefaultInheritanceAlpha
	if opts.InheritanceAlpha != nil {
		alpha = ClampInheritanceAlpha(*opts.InheritanceAlpha)
	}
	thresh := opts.EmbedDuplicateThreshold
	if thresh <= 0 || thresh >= 1 {
		thresh = DefaultEmbedDuplicateThreshold
	}
	timeout := opts.EmbedTimeout
	if timeout <= 0 {
		timeout = DefaultEmbedTimeoutSeconds * time.Second
	}
	st, err := sqlite.Open(abs, opts.Now)
	if err != nil {
		return nil, err
	}
	return &Vault{
		Dir:          abs,
		store:        st,
		now:          opts.Now,
		telemetry:    opts.Telemetry,
		inheritAlpha: alpha,
		embed:        opts.Embed,
		embedTimeout: timeout,
		embedThresh:  thresh,
	}, nil
}

// Close releases the underlying SQLite connection.
func (v *Vault) Close() error {
	if v == nil || v.store == nil {
		return nil
	}
	return v.store.Close()
}

func (v *Vault) instant() time.Time {
	if v.now == nil {
		return time.Now().UTC()
	}
	return v.now().UTC().Truncate(time.Second)
}

func (v *Vault) emit(event TelemetryEvent, traceID ...string) {
	if v.telemetry == nil {
		return
	}
	if event.At.IsZero() {
		event.At = v.instant()
	}
	if len(traceID) > 0 {
		event.TraceID = traceID[0]
	}
	_ = v.telemetry.Record(event)
}

// newTraceID returns a short random id that links all telemetry events of one
// logical operation (add → embed → reject). 12 hex chars keep the JSONL small
// while collisions stay negligible at personal-vault event rates.
func newTraceID() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(b[:])
}

func (v *Vault) load(id string) (*Record, error) {
	return v.store.GetRecord(id)
}

func (v *Vault) loadAll() ([]*Record, error) {
	return v.store.AllRecords(true)
}

func clampConfidence(c float64) float64 {
	if c < 0 {
		return 0
	}
	if c > MaxConfidence {
		return MaxConfidence
	}
	return c
}

func defaultConfidence(c float64) float64 {
	if c <= 0 {
		return DefaultConfidence
	}
	return clampConfidence(c)
}

// initialAddConfidence caps first-write confidence: tier 0.9 ("always") is reached via reinforce, not add.
func initialAddConfidence(c float64) float64 {
	c = defaultConfidence(c)
	if c >= 0.9 {
		return DefaultConfidence
	}
	return c
}

// ClampInheritanceAlpha keeps supersede inheritance_alpha in [0, 1].
func ClampInheritanceAlpha(alpha float64) float64 {
	if alpha < 0 {
		return 0
	}
	if alpha > 1 {
		return 1
	}
	return alpha
}

// inheritConfidence pulls new-rule confidence toward baseline by alpha of the gap.
// alpha 0 copies old; alpha 1 returns baseline. old at or below baseline stays at baseline.
func inheritConfidence(old, baseline, alpha float64) float64 {
	alpha = ClampInheritanceAlpha(alpha)
	gap := old - baseline
	if gap <= 0 {
		return baseline
	}
	inherited := old - gap*alpha
	if inherited < baseline {
		return baseline
	}
	return inherited
}

func cleanStrings(values []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, s := range values {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		key := strings.ToLower(s)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, s)
	}
	return out
}

func (v *Vault) cleanScope(scope []string) []string {
	values := cleanStrings(scope)
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, CanonicalScope(value))
	}
	return cleanStrings(out)
}

func (v *Vault) save(r *Record) error {
	return v.store.PutRecord(r)
}

// Add writes a new active rule. confidence <= 0 means 0.6.
func (v *Vault) Add(claim string, scope []string, text string, confidence float64) (*AddResult, error) {
	return v.AddRecord(claim, scope, text, confidence, nil, nil, nil, nil, "")
}

// AddWithSources is Add plus document sources for agent traceability.
func (v *Vault) AddWithSources(claim string, scope []string, text string, confidence float64, sources []DocRef) (*AddResult, error) {
	return v.AddRecord(claim, scope, text, confidence, nil, nil, nil, sources, "")
}

// AddRecord is Add plus optional relationship fields and query_local.
func (v *Vault) AddRecord(claim string, scope []string, text string, confidence float64, supersedes, related, conflicts []string, sources []DocRef, queryLocal string) (*AddResult, error) {
	start := time.Now()
	trace := newTraceID()
	claim = strings.TrimSpace(claim)
	if claim == "" {
		return nil, fmt.Errorf("claim is required")
	}
	scope = v.cleanScope(scope)
	if len(scope) == 0 {
		return nil, fmt.Errorf("scope is required")
	}
	if err := checkSensitiveText(
		guardedText{field: "claim", text: claim},
		guardedText{field: "text", text: text},
		guardedText{field: "query_local", text: queryLocal},
	); err != nil {
		v.emit(TelemetryEvent{Op: "write_rejected", Code: "privacy_rejected"}, trace)
		return nil, err
	}
	if err := v.checkSourcePaths(sources); err != nil {
		v.emit(TelemetryEvent{Op: "write_rejected", Code: "privacy_rejected"}, trace)
		return nil, err
	}
	duplicates, err := v.duplicateCandidates(claim, scope)
	if err != nil {
		return nil, err
	}
	// Semantic layer: embed the claim ONCE and reuse the vector for both the
	// duplicate check and the vector store — two ONNX round-trips per write
	// would double write latency for zero benefit. Blocking candidates reject
	// the write; advisory ones only ride along in the result so the caller
	// can arbitrate polarity conflicts.
	var advisory []DuplicateCandidate
	qvec, vecOK := v.embedClaim(claim, trace)
	duplicates, advisory, err = v.mergeEmbedDuplicatesVec(claim, qvec, vecOK, duplicates, trace)
	if err != nil {
		return nil, err
	}
	if len(duplicates) > 0 {
		v.emit(TelemetryEvent{Op: "write_rejected", Code: "duplicate"}, trace)
		return nil, &WriteGuardError{
			Code:       "duplicate",
			Message:    "similar active rule already exists",
			Candidates: duplicates,
			Hint:       "reinforce or supersede the matching rule",
		}
	}
	now := v.instant()
	id, err := v.store.NextID(now)
	if err != nil {
		return nil, err
	}
	rec := &Record{
		ID:                 id,
		Claim:              claim,
		Scope:              scope,
		Confidence:         initialAddConfidence(confidence),
		Status:             StatusActive,
		ReinforcementCount: 0,
		CreatedAt:          now,
		UpdatedAt:          now,
		Supersedes:         cleanStrings(supersedes),
		Related:            cleanStrings(related),
		ConflictsWith:      cleanStrings(conflicts),
		Sources:            cleanDocRefs(sources),
		QueryLocal:         MergeQueryLocalForStore(queryLocal, text),
		EvidenceLog: []Evidence{{
			At:   now,
			Kind: EvidenceOriginal,
			Text: text,
		}},
		Path: sqlite.DBPath(v.Dir),
	}
	if err := v.store.InsertRecord(rec); err != nil {
		return nil, err
	}
	if vecOK {
		v.storeEmbeddingVec(rec.ID, qvec, trace)
	}
	v.emit(TelemetryEvent{Op: "add", ScopeCount: len(scope), LatencyMS: time.Since(start).Milliseconds()}, trace)
	return &AddResult{ID: rec.ID, Confidence: rec.Confidence, Path: rec.Path, Similar: advisory}, nil
}

// Get returns the full record by id, including reverse links.
func (v *Vault) Get(id string) (*Record, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}
	rec, err := v.store.GetRecord(id)
	if err != nil {
		return nil, err
	}
	backlinks, err := v.store.Backlinks(id)
	if err != nil {
		return nil, err
	}
	rec.ReferencedBy = backlinks
	return rec, nil
}

func dropID(ids []string, id string) ([]string, bool) {
	if len(ids) == 0 {
		return ids, false
	}
	kept := make([]string, 0, len(ids))
	changed := false
	for _, x := range ids {
		if x == id {
			changed = true
			continue
		}
		kept = append(kept, x)
	}
	if !changed {
		return ids, false
	}
	return kept, true
}

// SourcesForIDs returns sources for the given rule ids.
func (v *Vault) SourcesForIDs(ids []string) (map[string][]DocRef, error) {
	want := map[string]struct{}{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			want[id] = struct{}{}
		}
	}
	if len(want) == 0 {
		return map[string][]DocRef{}, nil
	}
	filtered := make([]string, 0, len(want))
	for id := range want {
		filtered = append(filtered, id)
	}
	return v.store.SourcesForIDs(filtered)
}

// ConflictsAmong returns explicit conflicts_with edges among ids.
func (v *Vault) ConflictsAmong(ids []string) ([]ConflictSet, error) {
	pairs, err := v.store.ConflictPairs(ids)
	if err != nil {
		return nil, err
	}
	out := make([]ConflictSet, 0, len(pairs))
	for _, pair := range pairs {
		out = append(out, ConflictSet{A: pair[0], B: pair[1]})
	}
	return out, nil
}

// List returns summaries, optionally filtered by status.
func (v *Vault) List(status string, limit int) ([]ListItem, error) {
	return v.ListFilter(ListFilter{Status: status, Limit: limit})
}

// ListFilter returns summaries sliced by status, scope AND, confidence, query, and recency.
func (v *Vault) ListFilter(f ListFilter) ([]ListItem, error) {
	recs, err := v.loadAll()
	if err != nil {
		return nil, err
	}
	status := strings.TrimSpace(strings.ToLower(f.Status))
	scope := v.cleanScope(f.Scope)
	query := strings.TrimSpace(f.Query)
	var corpus []*Record
	if query != "" {
		corpus = recs
	}
	idx := buildIndex(corpus)
	var items []ListItem
	for _, r := range recs {
		if status != "" && string(r.Status) != status {
			continue
		}
		if f.MinConfidence > 0 && r.Confidence < f.MinConfidence {
			continue
		}
		if !f.Since.IsZero() && r.UpdatedAt.Before(f.Since) {
			continue
		}
		if len(scope) > 0 {
			_, matched := scopeScore(r.Scope, scope)
			if matched != len(scope) {
				continue
			}
		}
		if query != "" && idx.score(r, query) == 0 {
			continue
		}
		items = append(items, ListItem{
			ID:         r.ID,
			Title:      r.Title(),
			Confidence: r.Confidence,
			Status:     string(r.Status),
			Scope:      r.Scope,
		})
		if f.Limit > 0 && len(items) >= f.Limit {
			break
		}
	}
	if items == nil {
		items = []ListItem{}
	}
	return items, nil
}

// Show returns full records for a user-facing listing.
func (v *Vault) Show(limit int) ([]*Record, error) {
	recs, err := v.loadAll()
	if err != nil {
		return nil, err
	}
	sort.SliceStable(recs, func(i, j int) bool {
		order := func(s Status) int {
			switch s {
			case StatusActive:
				return 0
			case StatusDormant:
				return 1
			default:
				return 2
			}
		}
		if oi, oj := order(recs[i].Status), order(recs[j].Status); oi != oj {
			return oi < oj
		}
		return recs[i].ID > recs[j].ID
	})
	if limit > 0 && len(recs) > limit {
		recs = recs[:limit]
	}
	return recs, nil
}

// embedClaim embeds one claim and reports whether a vector is available.
// A nil embedder, timeout, or error yields ok=false so every caller degrades
// to the lexical path — writes must never fail because the sidecar is down.
func (v *Vault) embedClaim(claim string, traceID ...string) (vec []float32, ok bool) {
	if v.embed == nil {
		return nil, false
	}
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), v.embedTimeout)
	defer cancel()
	vecs, err := v.embed.Embed(ctx, []string{claim})
	if err != nil || len(vecs) == 0 {
		v.emit(TelemetryEvent{Op: "embed_unavailable", LatencyMS: time.Since(start).Milliseconds()}, traceID...)
		return nil, false
	}
	return vecs[0], true
}

// storeEmbedding encodes a rule's claim and stores its vector before the write
// returns.
//
// It is deliberately synchronous: imprint is often invoked as a short-lived CLI
// process, where a background goroutine would be killed at exit and silently
// leave the rule without a vector (which then defeats the next duplicate
// check). The call is bounded by embedTimeout, so a cold or crashed sidecar
// costs at most that delay and then degrades — the write itself never fails.
func (v *Vault) storeEmbedding(id, claim string, traceID ...string) {
	if vec, ok := v.embedClaim(claim, traceID...); ok {
		v.storeEmbeddingVec(id, vec, traceID...)
	}
}

// storeEmbeddingVec persists an already-computed vector. See storeEmbedding
// for why this must finish before the write returns.
func (v *Vault) storeEmbeddingVec(id string, vec []float32, traceID ...string) {
	start := time.Now()
	if err := v.store.UpsertRuleVector(id, v.embed.Model(), vec, time.Now().UTC()); err != nil {
		v.emit(TelemetryEvent{Op: "embed_unavailable", Code: "vector_write_failed", LatencyMS: time.Since(start).Milliseconds()}, traceID...)
		return
	}
	v.emit(TelemetryEvent{Op: "embed", LatencyMS: time.Since(start).Milliseconds()}, traceID...)
}

// Forget permanently deletes a record and strips inbound relationship ids.
func (v *Vault) Forget(id string) (*ForgetResult, error) {
	start := time.Now()
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}
	if _, err := v.load(id); err != nil {
		return nil, err
	}
	if err := v.store.DeleteRecord(id); err != nil {
		return nil, err
	}
	v.emit(TelemetryEvent{Op: "forget", LatencyMS: time.Since(start).Milliseconds()}, newTraceID())
	return &ForgetResult{Success: true}, nil
}

// ExportJSON writes every record as a JSON array.
func (v *Vault) ExportJSON(w io.Writer) error {
	recs, err := v.loadAll()
	if err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(true)
	return enc.Encode(recs)
}

// ExportJSONL writes one complete record per line for diffable audit exports.
func (v *Vault) ExportJSONL(w io.Writer) error {
	recs, err := v.loadAll()
	if err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	for _, rec := range recs {
		if err := enc.Encode(rec); err != nil {
			return err
		}
	}
	return nil
}

// Clear permanently deletes every rule in the vault.
func (v *Vault) Clear() error {
	return v.store.Clear()
}

// ImportRecords replaces vault contents with recs.
func (v *Vault) ImportRecords(recs []*Record) error {
	for _, r := range recs {
		r.Path = sqlite.DBPath(v.Dir)
	}
	return v.store.ImportRecords(recs)
}
