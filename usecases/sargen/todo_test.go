package sargen

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/checkmarble/marble-backend/usecases/sargen/formats/tracfin"
)

func TestBuildTodo_OrdersAndIncludesUnreviewedLowConfidence(t *testing.T) {
	doc := core.NewDocument()
	doc.Values["a.lowUnreviewed"] = core.Value{Value: "x", Origin: core.OriginLLM, Confidence: core.ConfidenceLow}
	doc.Values["b.lowReviewed"] = core.Value{Value: "y", Origin: core.OriginLLM, Confidence: core.ConfidenceLow, Reviewed: true}
	doc.Values["c.high"] = core.Value{Value: "z", Origin: core.OriginLLM, Confidence: core.ConfidenceHigh}

	issues := []core.Issue{
		{Path: "d.warn", Code: "test_warn", Message: "avertissement", Severity: core.SeverityWarning},
		{Path: "e.block", Code: "test_block", Message: "bloquant", Severity: core.SeverityBlocking},
	}

	todo := buildTodo(nil, nil, issues, doc, nil)
	require.Len(t, todo, 3, "two issues plus one unreviewed low-confidence value")

	require.Equal(t, TodoIssue, todo[0].Kind)
	require.Equal(t, TodoBlocking, todo[0].Severity, "blocking entries come first")
	require.Equal(t, core.Path("e.block"), todo[0].Paths[0])

	require.Equal(t, TodoLowConfidence, todo[1].Kind)
	require.Equal(t, TodoReview, todo[1].Severity, "unreviewed low-confidence values are review entries")
	require.Contains(t, todo[1].Paths, core.Path("a.lowUnreviewed"))

	require.Equal(t, TodoIssue, todo[2].Kind)
	require.Equal(t, TodoWarning, todo[2].Severity, "warnings come last")
	require.Equal(t, core.Path("d.warn"), todo[2].Paths[0])
}

// A question on a nil spec must not panic.
func TestBuildTodo_NilSpecWithQuestion(t *testing.T) {
	q := models.SarQuestion{Id: uuid.New(), Kind: models.SarQuestionKindQuestion, Paths: []string{"a.b"}, Text: "?"}
	todo := buildTodo(nil, nil, nil, nil, []models.SarQuestion{q})
	require.Len(t, todo, 1)
	require.Equal(t, TodoWarning, todo[0].Severity)
}

func TestBuildTodo_BlockingIssueOnComputedValueListsItsSources(t *testing.T) {
	doc := core.NewDocument()
	doc.Values["operations.operation[operation-1].montantTotal"] = core.Value{
		Value: "x", Origin: core.OriginComputed,
		Computation: &core.Computation{Name: "n", Inputs: []core.Path{"work.transactions[transactions-1].montant"}},
	}
	issues := []core.Issue{{
		Path: "operations.operation[operation-1].montantTotal", Code: "bad_decimal",
		Severity: core.SeverityBlocking, Message: "montant invalide",
	}}
	todo := buildTodo(nil, nil, issues, doc, nil)
	require.Len(t, todo, 1)
	assert.Equal(t, []core.Path{"operations.operation[operation-1].montantTotal", "work.transactions[transactions-1].montant"}, todo[0].Paths)
	assert.Contains(t, todo[0].Message, "corrigez les transactions sources")

	// A warning, or a value that is not computed, is left as is.
	warn := []core.Issue{{Path: issues[0].Path, Code: "x", Severity: core.SeverityWarning, Message: "m"}}
	assert.Len(t, buildTodo(nil, nil, warn, doc, nil)[0].Paths, 1)
}

func TestBuildTodo_ConfirmsOnlyTheFormatsComputedValues(t *testing.T) {
	spec, ok := tracfin.Spec(tracfin.CurrentVersion)
	require.True(t, ok)
	doc := core.NewDocument()
	doc.Values["operations.montantTotal"] = core.Value{Value: "10", Origin: core.OriginComputed}
	doc.Values["operations.dateDebut"] = core.Value{Value: "2026-01-01", Origin: core.OriginComputed}

	none := buildTodo(spec, nil, nil, doc, nil)
	assert.Empty(t, none, "a format that asks for no confirmation lists none")

	got := buildTodo(spec, []core.Path{"operations.montantTotal"}, nil, doc, nil)
	require.Len(t, got, 1)
	assert.Equal(t, []core.Path{"operations.montantTotal"}, got[0].Paths)
}

func TestBuildTodo_MissingLineFieldListsTheLinesSourceTransactions(t *testing.T) {
	doc := core.NewDocument()
	doc.Values["operations.operation[operation-1].nombreOperations"] = core.Value{
		Value: "1", Origin: core.OriginComputed,
		Computation: &core.Computation{Name: "n", Inputs: []core.Path{"work.transactions[transactions-1]"}},
	}
	issues := []core.Issue{{
		Path: "operations.operation[operation-1].emetteurTypeEmetteur", Code: core.CodeRequired,
		Severity: core.SeverityBlocking, Message: "émetteur manquant",
	}}
	todo := buildTodo(nil, nil, issues, doc, nil)
	require.Len(t, todo, 1)
	assert.Contains(t, todo[0].Paths, core.Path("work.transactions[transactions-1]"))
	assert.Contains(t, todo[0].Message, "corrigez les transactions sources")
}

// The confirm-to-check item of the total carries the format's note: credits,
// debits, the direction kept and the transactions without a direction.
func TestTodo_TotalToConfirmCarriesTheAmountNote(t *testing.T) {
	uc, f, declId := genReady(t, false)
	genEditDoc(t, f, declId, func(doc *core.Document) {
		for _, tx := range []struct{ montant, sens string }{{"100", "debit"}, {"30", "credit"}, {"7", ""}} {
			item := doc.AddItem("work.transactions", "transactions")
			for name, v := range map[string]string{"montant": tx.montant, "sens": tx.sens, "devise": "EUR", "date": "2026-03-01", "typeOperation": "OT10"} {
				if v != "" {
					doc.Values[item.Child(name)] = core.Value{Value: v, Origin: core.OriginManual}
				}
			}
		}
	})
	_, err := uc.UpdateFields(f.Ctx, declId, f.Repo.Declarations[declId].Version, []core.Fill{
		{Path: "work.transactions[transactions-1].paysOrigine", Value: "FR"},
		{Path: "work.sensMontantTotal", Value: "debit"}, // both directions: the analyst chooses, no default
	})
	require.NoError(t, err)

	st, err := uc.GetDeclaration(f.Ctx, declId)
	require.NoError(t, err)
	item, ok := findTodo(st.Todo, func(i TodoItem) bool { return i.Code == TodoCodeComputedToConfirm })
	require.True(t, ok)
	assert.Contains(t, item.Message, "Crédits : 30,00\u00a0€")
	assert.Contains(t, item.Message, "débits : 100,00\u00a0€")
	assert.Contains(t, item.Message, "Transactions sans sens : 1")
	assert.Equal(t, "100", st.Doc.Values["operations.montantTotal"].Value)
}
