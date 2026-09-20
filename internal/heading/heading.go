package heading

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var leadingEnum = regexp.MustCompile(`^(?:\d+(?:\.\d+)*[.)]?)\s+`)

const minSubstringRunes = 4

// Normalize maps a vault heading and an indexed ATX heading onto the same
// comparison key: markdown inline → plain text, lowercase, drop leading 5.2-style numbers.
func Normalize(s string) string {
	s = Plain(s)
	s = strings.ToLower(s)
	s = leadingEnum.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

// Match reports whether two headings name the same section after Normalize.
func Match(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}
	if strings.EqualFold(a, b) {
		return true
	}
	na, nb := Normalize(a), Normalize(b)
	if na == "" || nb == "" {
		return false
	}
	if na == nb {
		return true
	}
	if utf8.RuneCountInString(na) < minSubstringRunes || utf8.RuneCountInString(nb) < minSubstringRunes {
		return false
	}
	return strings.Contains(na, nb) || strings.Contains(nb, na)
}
