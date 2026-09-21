package imprint

import (
	"errors"
	"fmt"
	"time"

	"github.com/SteamedBread2333/imprint/internal/vault/model"
)

// AddResult is the --json shape for add.
type AddResult struct {
	ID         string  `json:"id"`
	Confidence float64 `json:"confidence"`
	Path       string  `json:"path"`
}

// FindHit is one find match.
type FindHit struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Scope         []string `json:"scope"`
	Confidence    float64  `json:"confidence"`
	Score         float64  `json:"score"`
	Status        string   `json:"status,omitempty"`
	SourcesCount  int      `json:"sources_count,omitempty"`
	EvidenceCount int      `json:"evidence_count,omitempty"`
	WakeCandidate bool     `json:"wake_candidate,omitempty"`
	QueryLocal    string   `json:"query_local,omitempty"`
}

// CompactFindHit is the default agent-facing recall shape.
type CompactFindHit struct {
	ID            string   `json:"id"`
	Claim         string   `json:"claim"`
	Scope         []string `json:"scope"`
	Confidence    float64  `json:"confidence"`
	Score         float64  `json:"score"`
	Status        string   `json:"status,omitempty"`
	SourcesCount  int      `json:"sources_count,omitempty"`
	EvidenceCount int      `json:"evidence_count,omitempty"`
	WakeCandidate bool     `json:"wake_candidate,omitempty"`
}

func (h FindHit) Compact() CompactFindHit {
	return CompactFindHit{
		ID:            h.ID,
		Claim:         h.Title,
		Scope:         h.Scope,
		Confidence:    h.Confidence,
		Score:         h.Score,
		Status:        h.Status,
		SourcesCount:  h.SourcesCount,
		EvidenceCount: h.EvidenceCount,
		WakeCandidate: h.WakeCandidate,
	}
}

// ConflictSet is one explicit conflict edge among returned rules.
type ConflictSet struct {
	A string `json:"a"`
	B string `json:"b"`
}

// ReinforceResult is the --json shape for reinforce.
type ReinforceResult struct {
	ID                 string  `json:"id"`
	Confidence         float64 `json:"confidence"`
	ReinforcementCount int     `json:"reinforcement_count"`
}

// SupersedeResult is the --json shape for supersede.
type SupersedeResult struct {
	ID              string `json:"id"`
	SupersededOldID string `json:"superseded_old_id"`
}

// ForgetResult is the --json shape for forget.
type ForgetResult struct {
	Success bool `json:"success"`
}

type Backlink = model.Backlink
type RuleStats = model.RuleStats
type RuleEvent = model.RuleEvent

// ListFilter is the optional slice for list.
type ListFilter struct {
	Status        string
	Scope         []string
	MinConfidence float64
	Query         string
	Since         time.Time
	Limit         int
}

// ListItem is one list row.
type ListItem struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Confidence float64  `json:"confidence"`
	Status     string   `json:"status,omitempty"`
	Scope      []string `json:"scope,omitempty"`
}

// SweepResult is the --json shape for sweep.
type SweepResult struct {
	Decayed  int `json:"decayed"`
	Archived int `json:"archived"`
}

type ReportRule struct {
	ID              string     `json:"id"`
	Claim           string     `json:"claim"`
	Status          string     `json:"status"`
	Confidence      float64    `json:"confidence"`
	RecallCount     int        `json:"recall_count"`
	LastRecalledAt  *time.Time `json:"last_recalled_at,omitempty"`
	LastConfirmedAt time.Time  `json:"last_confirmed_at"`
	Recommendation  string     `json:"recommendation,omitempty"`
}

type DuplicatePair struct {
	A     string  `json:"a"`
	B     string  `json:"b"`
	Score float64 `json:"score"`
}

type ReportResult struct {
	Since       time.Time       `json:"since"`
	EventCounts map[string]int  `json:"event_counts"`
	Duplicates  []DuplicatePair `json:"duplicates,omitempty"`
	Conflicts   []ConflictSet   `json:"conflicts,omitempty"`
	Rules       []ReportRule    `json:"rules"`
}

// ErrorBody is printed on stdout when --json commands fail.
type ErrorBody struct {
	Error string `json:"error"`
}

// DuplicateCandidate is an active rule that is too similar to a proposed add.
type DuplicateCandidate struct {
	ID    string  `json:"id"`
	Score float64 `json:"score"`
	Claim string  `json:"claim"`
}

// WriteGuardError is a structured privacy or duplicate rejection.
type WriteGuardError struct {
	Code       string               `json:"code"`
	Message    string               `json:"error"`
	Field      string               `json:"field,omitempty"`
	Kind       string               `json:"kind,omitempty"`
	Candidates []DuplicateCandidate `json:"candidates,omitempty"`
	Hint       string               `json:"hint,omitempty"`
}

func (e *WriteGuardError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s (%s)", e.Message, e.Field)
	}
	return e.Message
}

// ErrorPayload preserves structured write-guard details across adapters.
func ErrorPayload(err error) any {
	var guarded *WriteGuardError
	if errors.As(err, &guarded) {
		return guarded
	}
	return ErrorBody{Error: err.Error()}
}
