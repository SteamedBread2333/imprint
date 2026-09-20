package heading

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Plain turns an ATX heading or vault heading into comparable plain text:
// strip leading hashes, unwrap markdown inline (code, links, emphasis, images),
// unescape backslashes. It does not fetch or follow link targets.
func Plain(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimLeft(s, "#")
	s = strings.TrimSpace(s)
	return strings.Join(strings.Fields(inlinePlain(s)), " ")
}

func inlinePlain(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			r, size := utf8.DecodeRuneInString(s[i+1:])
			b.WriteRune(r)
			i += 1 + size
			continue
		}
		if s[i] == '`' {
			if n, inner, ok := consumeCodeSpan(s, i); ok {
				b.WriteString(inner)
				i += n
				continue
			}
		}
		if strings.HasPrefix(s[i:], "![") {
			if n, alt, ok := consumeLink(s, i+1); ok {
				b.WriteString(inlinePlain(alt))
				i += 1 + n
				continue
			}
		}
		if s[i] == '[' {
			if n, text, ok := consumeLink(s, i); ok {
				b.WriteString(inlinePlain(text))
				i += n
				continue
			}
		}
		if s[i] == '<' {
			if n, text, ok := consumeAngle(s, i); ok {
				b.WriteString(text)
				i += n
				continue
			}
		}
		if (s[i] == '*' || s[i] == '_') && emphasisOpen(s, i) {
			i += delimiterRun(s, i)
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func consumeCodeSpan(s string, i int) (n int, inner string, ok bool) {
	j := i
	for j < len(s) && s[j] == '`' {
		j++
	}
	ticks := j - i
	rest := s[j:]
	close := strings.Repeat("`", ticks)
	k := strings.Index(rest, close)
	if k < 0 {
		return 0, "", false
	}
	return ticks + k + ticks, rest[:k], true
}

func consumeLink(s string, i int) (n int, text string, ok bool) {
	if i >= len(s) || s[i] != '[' {
		return 0, "", false
	}
	depth := 1
	j := i + 1
	for j < len(s) {
		if s[j] == '\\' && j+1 < len(s) {
			j += 2
			continue
		}
		if s[j] == '[' {
			depth++
		}
		if s[j] == ']' {
			depth--
			if depth == 0 {
				text = s[i+1 : j]
				j++
				switch {
				case j < len(s) && s[j] == '(':
					if end, ok := skipBalanced(s, j, '(', ')'); ok {
						return end - i, text, true
					}
					return 0, "", false
				case j < len(s) && s[j] == '[':
					if end, ok := skipBalanced(s, j, '[', ']'); ok {
						return end - i, text, true
					}
					return 0, "", false
				default:
					return j - i, text, true
				}
			}
		}
		j++
	}
	return 0, "", false
}

func skipBalanced(s string, i int, open, close byte) (end int, ok bool) {
	if i >= len(s) || s[i] != open {
		return 0, false
	}
	depth := 1
	j := i + 1
	for j < len(s) {
		if s[j] == '\\' && j+1 < len(s) {
			j += 2
			continue
		}
		if s[j] == open {
			depth++
		}
		if s[j] == close {
			depth--
			if depth == 0 {
				return j + 1, true
			}
		}
		j++
	}
	return 0, false
}

func consumeAngle(s string, i int) (n int, text string, ok bool) {
	j := strings.IndexByte(s[i:], '>')
	if j < 0 {
		return 0, "", false
	}
	inner := s[i+1 : i+j]
	if strings.ContainsAny(inner, " \t\n") {
		return j + 1, "", true // HTML tag
	}
	return j + 1, inner, true // autolink label
}

func delimiterRun(s string, i int) int {
	d := s[i]
	n := 0
	for i+n < len(s) && s[i+n] == d {
		n++
	}
	return n
}

func emphasisOpen(s string, i int) bool {
	d := s[i]
	if d != '*' && d != '_' {
		return false
	}
	if i+1 >= len(s) {
		return false
	}
	if d == '_' && i > 0 {
		r, _ := utf8.DecodeLastRuneInString(s[:i])
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return false
		}
	}
	next, _ := utf8.DecodeRuneInString(s[i+delimiterRun(s, i):])
	return !unicode.IsSpace(next)
}
