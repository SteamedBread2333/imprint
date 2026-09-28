package shelves

import (
	"strings"

	"github.com/SteamedBread2333/imprint/internal/heading"
	"github.com/SteamedBread2333/imprint/internal/shelves/index"
	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

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

func resolveByPathHeading(st *index.Store, path, want string) imprint.ResolvedSource {
	path = strings.TrimSpace(path)
	want = strings.TrimSpace(want)
	var matches []index.Chunk
	for _, c := range st.Chunks {
		if c.Path != path {
			continue
		}
		matches = append(matches, c)
	}
	if len(matches) == 0 {
		return imprint.ResolvedSource{Path: path, Heading: want, OutOfIndex: true}
	}
	if want == "" {
		return chunkResolved(matches[0], false)
	}
	for _, c := range matches {
		if heading.Match(c.Heading, want) {
			return chunkResolved(c, false)
		}
	}
	return imprint.ResolvedSource{Path: path, Heading: want, HeadingUnresolved: true}
}

