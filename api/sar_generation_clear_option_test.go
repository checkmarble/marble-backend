package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// An option that clears values exposes them as clear; an option that writes
// nothing (leave as it is) has no clear, and the status of a question replaced
// by a review finding is exposed as is.
func TestHandleListQuestions_ClearingOptionAndReplacedStatus(t *testing.T) {
	declId := uuid.New()
	tx := core.Path("work.transactions[transactions-1]")
	linking := sargen.Question{
		Id: uuid.New(), DeclarationId: declId, Kind: models.SarQuestionKindQuestion,
		Status: models.SarQuestionStatusOpen, Paths: []core.Path{tx.Child("emetteurPersonne"), tx.Child("emetteurLibre")},
		Text: "Rattacher l'émetteur à Dupont ?",
		Options: []core.QuestionOption{
			{
				Label: "Rattacher à Dupont",
				Fills: []core.Fill{{Path: tx.Child("emetteurPersonne"), Value: "personnes.pp[pp-1]"}},
				Clear: []core.Path{tx.Child("emetteurLibre")},
			},
			{Label: "Laisser en texte libre"},
		},
	}
	replaced := sargen.Question{
		Id: uuid.New(), DeclarationId: declId, Kind: models.SarQuestionKindQuestion,
		Status: models.SarQuestionStatusReplaced, Paths: []core.Path{"operations.montantTotal"}, Text: "valeur manquante",
	}
	f := &fakeSarGenerationUsecase{Questions: []sargen.Question{linking, replaced}}
	r := newSarTestRouter(t, f)

	w := doSarRequest(r, http.MethodGet, "/sar-declarations/"+declId.String()+"/questions", nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var qs []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &qs))
	require.Len(t, qs, 2)
	opts := qs[0]["options"].([]any)
	assert.Equal(t, []any{string(tx.Child("emetteurLibre"))}, opts[0].(map[string]any)["clear"])
	assert.NotContains(t, opts[1], "clear")
	assert.Equal(t, "replaced", qs[1]["status"])
}
