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

// A question that proposes a new list item carries a provisional path and an
// option with no fill that declines the addition.

const sarAddPath = core.Path("context.natureSoupcon.natureSoupconPenale[+a]")

func sarAddQuestion(declId uuid.UUID) sargen.Question {
	return sargen.Question{
		Id: uuid.New(), DeclarationId: declId, Kind: models.SarQuestionKindQuestion,
		Status: models.SarQuestionStatusOpen, Paths: []core.Path{sarAddPath},
		Text: "Faut-il déclarer l'escroquerie en plus du blanchiment ?",
		Options: []core.QuestionOption{
			{Label: "Oui, déclarer", Fills: []core.Fill{{Path: sarAddPath, Value: "NSP07"}}},
			{Label: "Non, ne pas déclarer"},
		},
	}
}

func TestHandleListQuestions_AddQuestionAndDecliningOption(t *testing.T) {
	declId := uuid.New()
	f := &fakeSarGenerationUsecase{Questions: []sargen.Question{sarAddQuestion(declId)}}
	r := newSarTestRouter(t, f)

	w := doSarRequest(r, http.MethodGet, "/sar-declarations/"+declId.String()+"/questions", nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var qs []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &qs))
	require.Len(t, qs, 1)
	assert.Equal(t, []any{string(sarAddPath)}, qs[0]["paths"], "the provisional path is exposed as is")
	opts := qs[0]["options"].([]any)
	require.Len(t, opts, 2)
	add := opts[0].(map[string]any)
	require.Len(t, add["fills"], 1)
	assert.Equal(t, string(sarAddPath), add["fills"].([]any)[0].(map[string]any)["path"])
	decline := opts[1].(map[string]any)
	assert.Equal(t, "Non, ne pas déclarer", decline["label"])
	assert.Equal(t, []any{}, decline["fills"], "the declining option has an empty list of fills, not null")
}

func TestHandleChooseOption_AddOptionReturnsTheNewItem(t *testing.T) {
	declId, qid := uuid.New(), uuid.New()
	st := sarHTTPState(declId, uuid.New())
	st.Declaration.Version = 4
	key := st.Doc.AddItem("context.natureSoupcon.natureSoupconPenale", "natureSoupconPenale")
	st.Doc.Values[key] = core.Value{Value: "NSP07", Origin: core.OriginManual, Reviewed: true}
	f := &fakeSarGenerationUsecase{State: st}
	r := newSarTestRouter(t, f)

	w := doSarRequest(r, http.MethodPost, "/sar-questions/"+qid.String()+"/choose",
		map[string]any{"version": 3, "option_index": 0})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := sarHTTPDecode(t, w)
	assert.EqualValues(t, 4, body["version"])
	assert.Contains(t, w.Body.String(), "NSP07")
	calls := f.Called("ChooseQuestionOption")
	require.Len(t, calls, 1)
	assert.Equal(t, 3, calls[0].Args[1])
	assert.Equal(t, 0, calls[0].Args[2])
}

func TestHandleChooseOption_DecliningOptionKeepsTheVersion(t *testing.T) {
	declId, qid := uuid.New(), uuid.New()
	f := &fakeSarGenerationUsecase{State: sarHTTPState(declId, uuid.New())} // version 3, unchanged
	r := newSarTestRouter(t, f)

	w := doSarRequest(r, http.MethodPost, "/sar-questions/"+qid.String()+"/choose",
		map[string]any{"version": 3, "option_index": 1})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.EqualValues(t, 3, sarHTTPDecode(t, w)["version"], "nothing was written: the version is not bumped")
	calls := f.Called("ChooseQuestionOption")
	require.Len(t, calls, 1)
	assert.Equal(t, 1, calls[0].Args[2])
}

func TestHandleChooseOption_RefusedAddOptionIsAFrench400(t *testing.T) {
	qid := uuid.New()
	f := &fakeSarGenerationUsecase{}
	f.Fail("ChooseQuestionOption", sargen.NewIssuesError(models.SarCodeInvalidField, []core.Issue{{
		Path: "context.natureSoupcon.cadreLegal[+z]", Code: core.CodeTooManyItems,
		Message: "maximum number of items reached",
	}}))
	r := newSarTestRouter(t, f)

	w := doSarRequest(r, http.MethodPost, "/sar-questions/"+qid.String()+"/choose",
		map[string]any{"version": 3, "option_index": 0})

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "maximum number of items reached")
	assert.Contains(t, w.Body.String(), "issues")
}

func TestHandleListQuestions_MultiFillAddOption(t *testing.T) {
	declId := uuid.New()
	addr := core.Path("personnes.pp[pp-1].coordonnees.autreAdresse[+a]")
	q := sarAddQuestion(declId)
	q.Paths = []core.Path{addr + ".pays", addr + ".ville"}
	q.Options[0].Fills = []core.Fill{{Path: addr + ".pays", Value: "PT"}, {Path: addr + ".ville", Value: "Lisbonne"}}
	f := &fakeSarGenerationUsecase{Questions: []sargen.Question{q}}
	r := newSarTestRouter(t, f)

	w := doSarRequest(r, http.MethodGet, "/sar-declarations/"+declId.String()+"/questions", nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var qs []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &qs))
	fills := qs[0]["options"].([]any)[0].(map[string]any)["fills"].([]any)
	require.Len(t, fills, 2, "both fills of the add option share the provisional item")
	assert.Equal(t, string(addr)+".ville", fills[1].(map[string]any)["path"])
}

// A blocking question with an option that removes an item: severity and
// remove_item are exposed, and are absent on questions that do not use them (a
// front that only shows labels and posts choose keeps working).
func TestHandleListQuestions_RemovalOptionAndBlockingSeverity(t *testing.T) {
	declId := uuid.New()
	person := core.Path("personnes.pp[pp-1]")
	blocking := sargen.Question{
		Id: uuid.New(), DeclarationId: declId, Kind: models.SarQuestionKindQuestion, Severity: models.SarQuestionSeverityBlocking,
		Status: models.SarQuestionStatusOpen, Paths: []core.Path{person}, Text: "persons/p-A a été ajouté automatiquement",
		Options: []core.QuestionOption{{Label: "Déclarer persons/p-A comme personne impliquée"}, {Label: "Ne pas déclarer", RemoveItem: person}},
	}
	f := &fakeSarGenerationUsecase{Questions: []sargen.Question{blocking, sarAddQuestion(declId)}}
	r := newSarTestRouter(t, f)

	w := doSarRequest(r, http.MethodGet, "/sar-declarations/"+declId.String()+"/questions", nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var qs []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &qs))
	require.Len(t, qs, 2)
	assert.Equal(t, "blocking", qs[0]["severity"])
	opts := qs[0]["options"].([]any)
	keep, remove := opts[0].(map[string]any), opts[1].(map[string]any)
	assert.NotContains(t, keep, "remove_item")
	assert.Equal(t, []any{}, keep["fills"])
	assert.Equal(t, string(person), remove["remove_item"])
	assert.NotContains(t, qs[1], "severity")
	assert.NotContains(t, qs[1]["options"].([]any)[0], "remove_item")
}
