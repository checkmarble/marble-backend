package pure_utils

import (
	"strings"
	"unicode"

	fuzzy "github.com/paul-mannino/go-fuzzywuzzy"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

func Capitalize(str string) string {
	runes := []rune(str)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func Normalize(s string) string {
	result, _, _ := transform.String(norm.NFC, s)
	return result
}

func NormalizeAndRemoveDiacritics(s string) string {
	t := transform.Chain(
		norm.NFD,
		runes.Remove(runes.In(unicode.Mn)),
		norm.NFC,
	)
	result, _, _ := transform.String(t, s)
	return result
}

// - normalize
// - remove diacritics
// - set to lower case
// - keep only letters and numbers
// - keep non-ASCII characters
func CleanseString(s string) string {
	return strings.TrimSpace(fuzzy.Cleanse(NormalizeAndRemoveDiacritics(s), false))
}
