package pure_utils

import (
	"strings"
	"sync"
	"time"

	"github.com/adrg/strutil"
	"github.com/adrg/strutil/metrics"
	"github.com/biter777/countries"
	"github.com/hashicorp/golang-lru/v2/expirable"
)

const (
	FuzzyMatchThreshold = 0.85
	countryCacheSize    = 1000
	countryCacheTTL     = time.Hour
)

var (
	// countryCache caches fuzzy match results to avoid repeated computation
	countryCache     *expirable.LRU[string, string]
	countryCacheOnce sync.Once

	// countryNames is a pre-computed list of all country names in lowercase
	// for faster fuzzy matching
	countryNames     []countryNameEntry
	countryNamesOnce sync.Once
)

type countryNameEntry struct {
	lowerName string
	country   countries.CountryCode
}

func getCountryCache() *expirable.LRU[string, string] {
	countryCacheOnce.Do(func() {
		countryCache = expirable.NewLRU[string, string](countryCacheSize, nil, countryCacheTTL)
	})
	return countryCache
}

func getCountryNames() []countryNameEntry {
	countryNamesOnce.Do(func() {
		all := countries.All()
		countryNames = make([]countryNameEntry, 0, len(all))
		for _, c := range all {
			if c == countries.Unknown {
				continue
			}
			countryNames = append(countryNames, countryNameEntry{
				lowerName: strings.ToLower(c.Info().Name),
				country:   c,
			})
		}
	})
	return countryNames
}

// CountryToAlpha2 converts a country identifier (full name, Alpha-2, or Alpha-3 code)
// to its ISO 3166-1 Alpha-2 code. It handles misspellings and variations using
// fuzzy matching as a fallback.
//
// Returns the initial input if the country cannot be identified.
//
// Examples:
//
//	CountryToAlpha2("United States") // "US"
//	CountryToAlpha2("USA")           // "US"
//	CountryToAlpha2("US")            // "US"
//	CountryToAlpha2("Frence")        // "FR" (fuzzy match for typo)
//	CountryToAlpha2("")              // ""
func CountryToAlpha2(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}

	// Fast path: exact match (handles Alpha-2, Alpha-3, and standard English names)
	if c := countries.ByName(input); c != countries.Unknown {
		return c.Alpha2()
	}

	// Handle ISO 3166-2 subdivision codes (e.g., "CH-AI", "US-NY", "FR-75")
	// We only trigger this if the part before the hyphen is a 2 or 3 letter code
	// to avoid incorrectly splitting country names like "Guinea-Bissau".
	if strings.Contains(input, "-") {
		parts := strings.Split(input, "-")
		if len(parts) > 0 && len(parts[0]) >= 2 && len(parts[0]) <= 3 {
			if c := countries.ByName(parts[0]); c != countries.Unknown {
				return c.Alpha2()
			}
		}
	}

	cache := getCountryCache()
	if cached, ok := cache.Get(input); ok {
		return cached
	}

	// Fuzzy matching fallback
	result := fuzzyMatchCountry(input)
	if result == "" {
		// In case of fuzzy match failure, return the initial input
		result = input
	}

	cache.Add(input, result)

	return result
}

// fuzzyMatchCountry performs fuzzy string matching against all country names
// using the Jaro-Winkler algorithm, which is optimized for short strings.
func fuzzyMatchCountry(input string) string {
	inputLower := strings.ToLower(input)
	names := getCountryNames()

	// Jaro-Winkler is excellent for short strings like country names
	metric := metrics.NewJaroWinkler()

	bestMatch := countries.Unknown
	highestScore := 0.0

	for _, entry := range names {
		score := strutil.Similarity(inputLower, entry.lowerName, metric)
		if score > highestScore {
			highestScore = score
			bestMatch = entry.country
		}
	}

	if highestScore >= FuzzyMatchThreshold && bestMatch != countries.Unknown {
		return bestMatch.Alpha2()
	}

	return ""
}
