package index

const (
	// DefaultFindTopK is the document hit cap when find_top_k is unset.
	DefaultFindTopK = 2
	// DefaultSnippetRunes is the compact snippet rune cap when snippet_runes is unset.
	DefaultSnippetRunes = 80
	// DefaultMaxChunkLines flushes a section after this many body lines.
	DefaultMaxChunkLines = 12

	headingFieldWeight = 2.0
	pathFieldWeight    = 0.5
	bodyFieldWeight    = 1.0
)
