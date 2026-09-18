package model

import "time"

// Status is the lifecycle state of a rule.
type Status string

const (
	StatusActive     Status = "active"
	StatusDormant    Status = "dormant"
	StatusSuperseded Status = "superseded"
)

// EvidenceKind is why an evidence_log entry exists.
type EvidenceKind string

const (
	EvidenceOriginal  EvidenceKind = "original"
	EvidenceReinforce EvidenceKind = "reinforce"
	EvidenceSupersede EvidenceKind = "supersede"
)

const (
	DefaultConfidence    = 0.6
	MaxConfidence        = 0.95
	MinRecallConfidence  = 0.3
	DefaultDecayDays     = 90
	DefaultDecayAmount   = 0.05
	DefaultDormantThresh = 0.3
	DefaultTopK          = 5
	IDPrefix             = "r-"
)

// Evidence is one timestamped citation backing a rule.
type Evidence struct {
	At   time.Time    `json:"at"`
	Kind EvidenceKind `json:"kind"`
	Text string       `json:"text,omitempty"`
}

// DocRef points a rule at a workspace document (path, optional heading, optional chunk id).
type DocRef struct {
	Path    string `json:"path,omitempty"`
	Heading string `json:"heading,omitempty"`
	Chunk   string `json:"chunk,omitempty"`
}

// Backlink is another rule that points at this one.
type Backlink struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
}

// ReferencedRule is an imprint whose vault sources point at a document path or chunk.
type ReferencedRule struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Claim string `json:"claim,omitempty"`
}

// Record is one imprint rule in vault.db.
type Record struct {
	ID                 string     `json:"id"`
	Claim              string     `json:"claim"`
	Scope              []string   `json:"scope"`
	Confidence         float64    `json:"confidence"`
	Status             Status     `json:"status"`
	ReinforcementCount int        `json:"reinforcement_count"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	LastTouchedAt      time.Time  `json:"last_touched_at"`
	Supersedes         []string   `json:"supersedes"`
	Related            []string   `json:"related"`
	ConflictsWith      []string   `json:"conflicts_with"`
	Sources            []DocRef   `json:"sources,omitempty"`
	EvidenceLog        []Evidence `json:"evidence_log"`
	Body               string     `json:"body,omitempty"`
	Path               string     `json:"path,omitempty"`
	ReferencedBy       []Backlink `json:"referenced_by,omitempty"`
}

func (r *Record) Title() string {
	return r.Claim
}
