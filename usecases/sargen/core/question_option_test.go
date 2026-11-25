package core_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// The new option and question fields are optional: a plain option serializes
// exactly as before, and an old stored option decodes without them.
func TestQuestionOption_NewFieldsAreOptional(t *testing.T) {
	raw, err := json.Marshal(core.QuestionOption{Label: "Oui"})
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "remove_item")

	raw, err = json.Marshal(core.QuestionDraft{Text: "q"})
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "blocking")

	var opt core.QuestionOption
	require.NoError(t, json.Unmarshal([]byte(`{"label":"Non","fills":[]}`), &opt))
	assert.Empty(t, opt.RemoveItem)

	raw, err = json.Marshal(core.QuestionOption{Label: "Ne pas déclarer", RemoveItem: "personnes.pp[pp-1]"})
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"remove_item":"personnes.pp[pp-1]"`)
}
