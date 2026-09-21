package imprint

import (
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
	Dir       string
	store     *sqlite.Store
	now       func() time.Time
	telemetry EventSink
}

// OpenOptions configures a vault. Construction is immutable after Open.
type OpenOptions struct {
	Dir       string
	Now       func() time.Time
	Telemetry EventSink
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
	st, err := sqlite.Open(abs, opts.Now)
	if err != nil {
		return nil, err
	}
	return &Vault{Dir: abs, store: st, now: opts.Now, telemetry: opts.Telemetry}, nil
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

func (v *Vault) emit(event TelemetryEvent) {
	if v.telemetry == nil {
		return
	}
	if event.At.IsZero() {
		event.At = v.instant()
	}
	_ = v.telemetry.Record(event)
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
		v.emit(TelemetryEvent{Op: "write_rejected", Code: "privacy_rejected"})
		return nil, err
	}
	if err := v.checkSourcePaths(sources); err != nil {
		v.emit(TelemetryEvent{Op: "write_rejected", Code: "privacy_rejected"})
		return nil, err
	}
	duplicates, err := v.duplicateCandidates(claim, scope)
	if err != nil {
		return nil, err
	}
	if len(duplicates) > 0 {
		v.emit(TelemetryEvent{Op: "write_rejected", Code: "duplicate"})
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
	v.emit(TelemetryEvent{Op: "add", ScopeCount: len(scope), LatencyMS: time.Since(start).Milliseconds()})
	return &AddResult{ID: rec.ID, Confidence: rec.Confidence, Path: rec.Path}, nil
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
	v.emit(TelemetryEvent{Op: "forget", LatencyMS: time.Since(start).Milliseconds()})
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
