package imprint

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

// VizResult is the --json shape for viz.
type VizResult struct {
	Path       string `json:"path"`
	RulesCount int    `json:"rules_count"`
	SizeBytes  int64  `json:"size_bytes"`
	Mermaid    string `json:"mermaid,omitempty"`
}

// ErrorBody is printed on stdout when --json commands fail.
type ErrorBody struct {
	Error string `json:"error"`
}
