package pure_utils

import (
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/adrg/strutil/metrics"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

func Normalize(s string) string {
	result, _, _ := transform.String(norm.NFC, s)
	return result
}

func normalizeAndRemoveDiacritics(s string) string {
	t := transform.Chain(
		norm.NFD,
		runes.Remove(runes.In(unicode.Mn)),
		norm.NFC,
	)
	result, _, _ := transform.String(t, s)
	return result
}

func onlyLettersAndNumbers(s string) string {
	var result strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			result.WriteRune(r)
		} else {
			result.WriteRune(' ')
		}
	}
	return result.String()
}

// - normalize
// - remove diacritics
// - set to lower case
// - only letters and numbers
func cleanseString(s string) string {
	return strings.TrimSpace(strings.ToLower(
		onlyLettersAndNumbers(normalizeAndRemoveDiacritics(s))))
}

// Converts a string into a set (map) of unique words.
func stringToSet(s string) map[string]bool {
	set := make(map[string]bool)
	tokens := strings.Fields(s)
	for _, token := range tokens {
		set[token] = true
	}
	return set
}

// Calculates the intersection of two sets.
func intersect(set1, set2 map[string]bool) map[string]bool {
	intersection := make(map[string]bool)
	for key := range set1 {
		if _, exists := set2[key]; exists {
			intersection[key] = true
		}
	}
	return intersection
}

// Calculates the difference of two sets (set1 - set2).
func difference(set1, set2 map[string]bool) map[string]bool {
	diff := make(map[string]bool)
	for key := range set1 {
		if _, exists := set2[key]; !exists {
			diff[key] = true
		}
	}
	return diff
}

func setToSlice(set map[string]bool) []string {
	slice := make([]string, 0, len(set))
	for key := range set {
		slice = append(slice, key)
	}
	return slice
}

func similarityRatioFloat(s1, s2 string) float64 {
	sumOfLen := len(s1) + len(s2)
	if sumOfLen == 0 {
		return 1
	}

	lev := metrics.NewLevenshtein()
	// There are different flavors of Levenshtein distance. We need a replace cost of 2 if two completely
	// different strings (no shared letter in same position) of same size are to have a similarity of 0 and not 50%
	// Not all implementations available in open-source conform with this (or expose the option).
	lev.InsertCost = 1
	lev.ReplaceCost = 2
	lev.DeleteCost = 1
	editDistance := lev.Distance(s1, s2)
	return 1 - float64(editDistance)/float64(sumOfLen)
}

func similarityRatio(s1, s2 string) int {
	return int(math.Round(similarityRatioFloat(s1, s2) * 100))
}

func DirectSimilarity(s1, s2 string) int {
	s1 = cleanseString(s1)
	s2 = cleanseString(s2)
	return similarityRatio(s1, s2)
}

// Calculates the similarity ratio between two strings.
func BagOfWordsSimilarity(s1, s2 string) int {
	s1 = cleanseString(s1)
	s2 = cleanseString(s2)

	set1 := stringToSet(s1)
	set2 := stringToSet(s2)

	intersection := intersect(set1, set2)
	diff1to2 := difference(set1, set2)
	diff2to1 := difference(set2, set1)

	intersectionSlice := setToSlice(intersection)
	diff1to2Slice := setToSlice(diff1to2)
	diff2to1Slice := setToSlice(diff2to1)

	sort.Strings(intersectionSlice)
	sort.Strings(diff1to2Slice)
	sort.Strings(diff2to1Slice)

	intersectionBag := strings.Join(intersectionSlice, " ")
	diff1to2Bag := strings.Trim(intersectionBag+" "+strings.Join(diff1to2Slice, " "), " ")
	diff2to1Bag := strings.Trim(intersectionBag+" "+strings.Join(diff2to1Slice, " "), " ")

	ratio1 := similarityRatio(intersectionBag, diff1to2Bag)
	ratio2 := similarityRatio(intersectionBag, diff2to1Bag)
	ratio3 := similarityRatio(diff1to2Bag, diff2to1Bag)

	out := ratio1
	if ratio2 > out {
		out = ratio2
	}
	if ratio3 > out {
		out = ratio3
	}
	return out
}
