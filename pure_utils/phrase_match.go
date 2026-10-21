package pure_utils

import (
	"math"
	"strings"
	"unicode/utf8"

	"github.com/adrg/strutil/metrics"
)

const (
	// Maximum number of unselected words allowed between the first and last matched positions.
	phraseMatchMaxGaps = 3
	// Lowers the score for each unselected word between the first and last matched positions.
	phraseMatchGapPenalty = 0.2
	// Multiplies the score when the words appear in a different order than in the phrase.
	phraseMatchOrderPenalty = 0.85
	// Words shorter than this (articles, prepositions, elided "l'"…) are ignored
	// in both inputs, unless all phrase words are shorter than this.
	phraseMatchMinWordLength = 3
)

// PhraseSimilarity returns how well phrase is found inside text, from 0 to 100.
//
// Every retained phrase word must have an accepted fuzzy match in the same
// window. Different phrase words may reuse the same text position. A window
// scores zero if any phrase word has no match or if more than phraseMatchMaxGaps
// unselected words lie between the first and last selected positions.
// Words shorter than phraseMatchMinWordLength are ignored in both inputs,
// unless all phrase words are shorter than this.
//
// Matching runs in two steps:
//
//  1. Word lookup: each phrase word is compared to each word of the text with
//     a Levenshtein distance. The number of edits allowed depends on the
//     phrase word's length, as with Elasticsearch's AUTO fuzziness: 0 for 1–2
//     letters, 1 for 3–5, 2 for longer words.
//  2. Proximity: a window of up to the retained phrase word count plus
//     phraseMatchMaxGaps slides over the filtered text. Each phrase word
//     independently selects its highest-similarity match in the window;
//     ties favor the earliest position. Selection does not optimize the
//     combined gap and order penalties. The best-scoring window wins.
//
// A window's score is
//
//	avg(similarity) / (1 + gapPenalty·gaps) × (orderPenalty if out of order)
//
// where similarity is 1 − distance / max(len) for each matched word.
func PhraseSimilarity(text, phrase string) int {
	textWords := strings.Fields(cleanseString(text))
	phraseWords := strings.Fields(cleanseString(phrase))

	// Keep short words when they are all the phrase has, so "a b" can still match.
	if significant := phraseSignificantWords(phraseWords); len(significant) > 0 {
		phraseWords = significant
		textWords = phraseSignificantWords(textWords)
	}

	if len(textWords) == 0 || len(phraseWords) == 0 {
		return 0
	}

	// sims[i][p] is the similarity between phrase word i and text word p, 0 if
	// they are too far apart.
	sims := make([][]float64, len(phraseWords))
	for i, pw := range phraseWords {
		sims[i] = phraseWordSimilarities(pw, textWords)
	}

	width := len(phraseWords) + phraseMatchMaxGaps
	best := 0.0

	for lo := range textWords {
		best = max(best, phraseWindowScore(sims, lo, min(lo+width, len(textWords))))
	}

	return int(math.Round(best * 100))
}

func phraseSignificantWords(words []string) []string {
	out := make([]string, 0, len(words))
	for _, w := range words {
		if utf8.RuneCountInString(w) >= phraseMatchMinWordLength {
			out = append(out, w)
		}
	}

	return out
}

// phraseWordSimilarities compares word to every text word, computing the
// distance at most once per distinct text word.
func phraseWordSimilarities(word string, textWords []string) []float64 {
	maxEdits := autoFuzziness(word)
	wordLen := utf8.RuneCountInString(word)
	lev := metrics.NewLevenshtein()

	sims := make([]float64, len(textWords))
	seen := map[string]float64{}

	for p, tw := range textWords {
		sim, ok := seen[tw]
		if !ok {
			// Cheap pre-check: the length difference alone needs that many edits.
			twLen := utf8.RuneCountInString(tw)
			if wordLen-twLen <= maxEdits && twLen-wordLen <= maxEdits {
				if d := lev.Distance(word, tw); d <= maxEdits {
					sim = 1 - float64(d)/float64(max(wordLen, twLen))
				}
			}

			seen[tw] = sim
		}

		sims[p] = sim
	}

	return sims
}

// phraseWindowScore scores text words [lo, hi), each phrase word taking its
// best match in that range. It returns 0 if a phrase word has no match.
func phraseWindowScore(sims [][]float64, lo, hi int) float64 {
	distinct := map[int]struct{}{}
	first, last := hi, lo
	ordered := true
	total := 0.0
	prev := -1

	for _, wordSims := range sims {
		pos := -1
		for p := lo; p < hi; p++ {
			if wordSims[p] > 0 && (pos == -1 || wordSims[p] > wordSims[pos]) {
				pos = p
			}
		}

		if pos == -1 {
			return 0
		}

		distinct[pos] = struct{}{}
		first, last = min(first, pos), max(last, pos)
		total += wordSims[pos]

		if pos < prev {
			ordered = false
		}

		prev = pos
	}

	gaps := last - first + 1 - len(distinct)
	if gaps > phraseMatchMaxGaps {
		return 0
	}

	score := total / float64(len(sims)) / (1 + phraseMatchGapPenalty*float64(gaps))
	if !ordered {
		score *= phraseMatchOrderPenalty
	}

	return score
}

func autoFuzziness(word string) int {
	switch n := utf8.RuneCountInString(word); {
	case n <= 2:
		return 0
	case n <= 5:
		return 1
	default:
		return 2
	}
}
