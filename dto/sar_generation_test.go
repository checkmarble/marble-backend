package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

func TestAdaptSarDocumentDto_ItemsAndSources(t *testing.T) {
	doc := core.NewDocument()
	withSources := doc.AddItem("personnes.pp", "pp", core.ObjectRef{Table: "accounts", ObjectID: "a1"})
	withoutSources := doc.AddItem("personnes.pp", "pp")

	out := AdaptSarDocumentDto(doc)

	assert.Equal(t, map[string][]string{"personnes.pp": {"pp-1", "pp-2"}}, out.Items)
	assert.Equal(t, map[string][]SarObjectRefDto{
		string(withSources): {{Table: "accounts", ObjectId: "a1"}},
	}, out.ItemSources)
	assert.NotContains(t, out.ItemSources, string(withoutSources))
}

// Every value the analyst clears, from the form or with a question option,
// has the manual origin and an empty value: the DTO shows it as cleared.
func TestAdaptSarValueDto_ManualClearIsCleared(t *testing.T) {
	assert.True(t, adaptSarValueDto(core.Value{Origin: core.OriginManual, Reviewed: true}).Cleared)
	assert.False(t, adaptSarValueDto(core.Value{Value: "x", Origin: core.OriginManual}).Cleared)
	assert.False(t, adaptSarValueDto(core.Value{Origin: core.OriginLLM}).Cleared)
}
