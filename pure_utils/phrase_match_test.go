package pure_utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPhraseSimilarity(t *testing.T) {
	examples := []struct {
		text     string
		phrase   string
		expected int
	}{
		{"L'appel à mon ami", "Appel humanitaire", 0},
		{"Un appel humanitaire", "Appel humanitaire", 100},
		{"L'appel humanitaire", "Appel humanitaire", 100},
		{"Un apel humanitère urgent", "Appel humanitaire", 81},
		// "et" is ignored, so only "urgent" counts as a gap
		{"Un appel urgent et humanitaire a été lancé", "Appel humanitaire", 83},
		{"humanitaire, cet appel", "Appel humanitaire", 71},
		{"Appel lancé par le maire, sans rien de très humanitaire", "Appel humanitaire", 0},
		{"", "Appel humanitaire", 0},
		{"Appel humanitaire", "", 0},
		// Picks the tightest window
		{"La crise, dit-on, est économique ; la crise économique s'aggrave", "crise économique", 100},
		// Short words are ignored
		{"appel de l'aide", "appel à l'aide", 100},
		// Unless the phrase only has short words, which must then match exactly
		{"il y a un an", "un an", 100},
		{"il y a un mois", "un an", 0},
	}

	for _, example := range examples {
		t.Run(example.text+" vs "+example.phrase, func(t *testing.T) {
			assert.Equal(t, example.expected, PhraseSimilarity(example.text, example.phrase))
		})
	}
}
