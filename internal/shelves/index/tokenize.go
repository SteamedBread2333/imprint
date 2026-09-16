package index

import (
	"strings"
	"unicode"
)

func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r)
}

// Tokenize splits text for BM25 (CJK-aware).
func Tokenize(s string) []string {
	var out []string
	var buf []rune
	kind := 0
	flush := func() {
		if len(buf) == 0 {
			return
		}
		if kind == 2 {
			for i := range buf {
				out = append(out, string(buf[i]))
				if i+1 < len(buf) {
					out = append(out, string(buf[i:i+2]))
				}
			}
		} else if len(buf) > 1 {
			out = append(out, string(buf))
		}
		buf = buf[:0]
		kind = 0
	}
	for _, r := range strings.ToLower(s) {
		switch {
		case isCJK(r):
			if kind == 1 {
				flush()
			}
			kind = 2
			buf = append(buf, r)
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			if kind == 2 {
				flush()
			}
			kind = 1
			buf = append(buf, r)
		default:
			flush()
		}
	}
	flush()
	return out
}
