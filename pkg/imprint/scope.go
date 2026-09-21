package imprint

import (
	"strings"

	enry "github.com/go-enry/go-enry/v2"
)

// CanonicalScope maps a scope tag onto GitHub Linguist's language name when
// the tag is a known language alias or an unambiguous extension. Non-language
// tags such as naming or frontend are unchanged.
func CanonicalScope(tag string) string {
	tag = strings.ToLower(strings.TrimSpace(tag))
	if tag == "" {
		return ""
	}
	lang, ok := enry.GetLanguageByAlias(tag)
	if !ok {
		if ext, safe := enry.GetLanguageByExtension("x." + tag); safe && ext != "" {
			lang = ext
		}
	}
	if lang == "" || lang == enry.OtherLanguage {
		return tag
	}
	if group := enry.GetLanguageGroup(lang); group != "" {
		lang = group
	}
	return languageScopeTag(lang)
}

func languageScopeTag(lang string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(lang), " ", "-"))
}
