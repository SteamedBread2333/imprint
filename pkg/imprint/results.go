package imprint

import (
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
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Scope      []string `json:"scope"`
	Confidence float64  `json:"confidence"`
	Score      float64  `json:"score"`
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

// ErrorBody is printed on stdout when --json commands fail.
type ErrorBody struct {
	Error string `json:"error"`
}
