package shelves

import (
	"github.com/SteamedBread2333/imprint/internal/shelves/index"
	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

// ReferencedRules returns imprints whose vault sources point at this chunk (no markdown required).
func ReferencedRules(v *imprint.Vault, c index.Chunk) ([]imprint.ReferencedRule, error) {
	if v == nil {
		return nil, nil
	}
	return v.RulesReferencingDoc(c.Path, c.Heading, c.ID)
}
