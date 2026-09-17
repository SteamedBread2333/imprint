package shelves

import (
	"strings"

	"github.com/SteamedBread2333/imprint/internal/shelves/index"
	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

// CitedRule is a vault rule referenced from document text.
type CitedRule struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	LineNo int    `json:"line_no,omitempty"`
}

// ResolveSources maps vault DocRefs to shelves excerpts for agent recall.
func ResolveSources(st *index.Store, refs []imprint.DocRef) []imprint.ResolvedSource {
	if len(refs) == 0 {
		return nil
	}
	out := make([]imprint.ResolvedSource, 0, len(refs))
	for _, ref := range refs {
		out = append(out, resolveOne(st, ref))
	}
	return out
}

func resolveOne(st *index.Store, ref imprint.DocRef) imprint.ResolvedSource {
	ref.Path = strings.TrimSpace(ref.Path)
	ref.Path = strings.TrimPrefix(ref.Path, "./")
	ref.Path = strings.ReplaceAll(ref.Path, "\\", "/")
	ref.Heading = strings.TrimSpace(ref.Heading)
	ref.Chunk = strings.TrimSpace(ref.Chunk)

	if st == nil || len(st.Chunks) == 0 {
		return imprint.ResolvedSource{
			Path:       ref.Path,
			Heading:    ref.Heading,
			OutOfIndex: ref.Path != "" || ref.Chunk != "",
		}
	}

	if ref.Chunk != "" {
		if c, ok := st.ChunkByID(ref.Chunk); ok {
			return chunkResolved(c, false)
		}
		res := resolveByPathHeading(st, ref.Path, ref.Heading)
		if res.ChunkID != "" {
			res.StaleChunk = true
			return res
		}
		return imprint.ResolvedSource{
			Path:       ref.Path,
			Heading:    ref.Heading,
			StaleChunk: true,
			OutOfIndex: ref.Path == "",
		}
	}

	if ref.Path == "" {
		return imprint.ResolvedSource{}
	}
	return resolveByPathHeading(st, ref.Path, ref.Heading)
}

func chunkResolved(c index.Chunk, stale bool) imprint.ResolvedSource {
	return imprint.ResolvedSource{
		Path:       c.Path,
		Heading:    c.Heading,
		ChunkID:    c.ID,
		Snippet:    index.Snippet(c.Text, 240),
		LineStart:  c.LineStart,
		LineEnd:    c.LineEnd,
		StaleChunk: stale,
	}
}

func resolveByPathHeading(st *index.Store, path, heading string) imprint.ResolvedSource {
	path = strings.TrimSpace(path)
	heading = strings.TrimSpace(heading)
	var matches []index.Chunk
	for _, c := range st.Chunks {
		if c.Path != path {
			continue
		}
		matches = append(matches, c)
	}
	if len(matches) == 0 {
		return imprint.ResolvedSource{Path: path, Heading: heading, OutOfIndex: true}
	}
	if heading == "" {
		return chunkResolved(matches[0], false)
	}
	headLower := strings.ToLower(heading)
	for _, c := range matches {
		if strings.EqualFold(strings.TrimSpace(c.Heading), headLower) || strings.EqualFold(c.Heading, heading) {
			return chunkResolved(c, false)
		}
	}
	for _, c := range matches {
		if strings.Contains(strings.ToLower(c.Heading), headLower) {
			return chunkResolved(c, false)
		}
	}
	return chunkResolved(matches[0], false)
}

// CitedRules returns rules referenced from a chunk via [[r-…]] or [imprint:r-…].
func CitedRules(st *index.Store, chunkID string) []CitedRule {
	if st == nil {
		return nil
	}
	var out []CitedRule
	for _, ref := range st.RuleRefs {
		if ref.ChunkID != chunkID {
			continue
		}
		out = append(out, CitedRule{ID: ref.RuleID, Kind: "cited_by", LineNo: ref.LineNo})
	}
	return out
}
