package sargen

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// compTxFills returns the required fields of the transaction at tx: 100 EUR
// debit, OT10 (virement) from FR to DE.
func compTxFills(tx core.Path, montant, date string) []core.Fill {
	return []core.Fill{
		{Path: tx.Child("montant"), Value: montant},
		{Path: tx.Child("date"), Value: date},
		{Path: tx.Child("sens"), Value: "debit"},
		{Path: tx.Child("devise"), Value: "EUR"},
		{Path: tx.Child("typeOperation"), Value: "OT10"},
		{Path: tx.Child("paysOrigine"), Value: "FR"},
		{Path: tx.Child("paysArrivee"), Value: "DE"},
	}
}

// compOnlyLine returns the only operation line of the document.
func compOnlyLine(t *testing.T, doc *core.Document) core.Path {
	t.Helper()
	keys := doc.Items["operations.operation"]
	require.Len(t, keys, 1, "one operation line is expected")
	return core.Path("operations.operation[" + keys[0] + "]")
}

func compDecimalOf(t *testing.T, s string) float64 {
	t.Helper()
	v, err := strconv.ParseFloat(s, 64)
	require.NoError(t, err, "decimal value %q", s)
	return v
}

func TestManualTransactionEditRecomputesOperations(t *testing.T) {
	uc, f := newTestUsecase(t)
	st := createFresh(t, uc, f)
	id := st.Declaration.Id

	tx1, st, err := uc.AddItem(f.Ctx, id, st.Declaration.Version, "work.transactions")
	require.NoError(t, err)
	st, err = uc.UpdateFields(f.Ctx, id, st.Declaration.Version, compTxFills(tx1, "100.00", "2026-03-10"))
	require.NoError(t, err)

	tx2, st, err := uc.AddItem(f.Ctx, id, st.Declaration.Version, "work.transactions")
	require.NoError(t, err)
	st, err = uc.UpdateFields(f.Ctx, id, st.Declaration.Version, compTxFills(tx2, "200.00", "2026-04-01"))
	require.NoError(t, err)

	// The transactions are added and edited by hand: the operation line and the
	// total follow them in the same write.
	line := compOnlyLine(t, st.Doc)
	assert.InDelta(t, 300, compDecimalOf(t, st.Doc.Values[line.Child("montantTotal")].Value), 0.001)
	assert.InDelta(t, 300, compDecimalOf(t, st.Doc.Values["operations.montantTotal"].Value), 0.001)

	// Correcting the amount of a transaction by hand recomputes both.
	st, err = uc.UpdateFields(f.Ctx, id, st.Declaration.Version, []core.Fill{
		{Path: tx1.Child("montant"), Value: "400.00"},
	})
	require.NoError(t, err)

	line = compOnlyLine(t, st.Doc)
	assert.InDelta(t, 600, compDecimalOf(t, st.Doc.Values[line.Child("montantTotal")].Value), 0.001)
	assert.InDelta(t, 600, compDecimalOf(t, st.Doc.Values["operations.montantTotal"].Value), 0.001)
	assert.Equal(t, core.OriginComputed, st.Doc.Values["operations.montantTotal"].Origin)
}

func TestManualTotalIsNotOverwrittenByCompute(t *testing.T) {
	uc, f := newTestUsecase(t)
	st := createFresh(t, uc, f)
	id := st.Declaration.Id

	tx1, st, err := uc.AddItem(f.Ctx, id, st.Declaration.Version, "work.transactions")
	require.NoError(t, err)
	st, err = uc.UpdateFields(f.Ctx, id, st.Declaration.Version, compTxFills(tx1, "100.00", "2026-03-10"))
	require.NoError(t, err)

	// The analyst corrects the computed total by hand.
	st, err = uc.UpdateFields(f.Ctx, id, st.Declaration.Version, []core.Fill{
		{Path: "operations.montantTotal", Value: "999.00"},
	})
	require.NoError(t, err)
	require.Equal(t, core.OriginManual, st.Doc.Values["operations.montantTotal"].Origin)

	// A later transaction edit recomputes the lines but keeps the analyst's total.
	st, err = uc.UpdateFields(f.Ctx, id, st.Declaration.Version, []core.Fill{
		{Path: tx1.Child("montant"), Value: "150.00"},
	})
	require.NoError(t, err)

	total := st.Doc.Values["operations.montantTotal"]
	assert.Equal(t, "999.00", total.Value, "a total corrected by hand is not overwritten")
	assert.Equal(t, core.OriginManual, total.Origin)

	line := compOnlyLine(t, st.Doc)
	assert.InDelta(t, 150, compDecimalOf(t, st.Doc.Values[line.Child("montantTotal")].Value), 0.001,
		"the line itself is still recomputed")
}

func TestChooseQuestionOption_RecomputesOperations(t *testing.T) {
	uc, f := newTestUsecase(t)
	st := createFresh(t, uc, f)
	id := st.Declaration.Id

	tx, st, err := uc.AddItem(f.Ctx, id, st.Declaration.Version, "work.transactions")
	require.NoError(t, err)
	st, err = uc.UpdateFields(f.Ctx, id, st.Declaration.Version, compTxFills(tx, "100.00", "2026-03-10"))
	require.NoError(t, err)

	q := f.SeedQuestion(t, id, []core.Path{tx.Child("montant")}, "Quel montant ?", []core.QuestionOption{{
		Label: "Centimes convertis",
		Fills: []core.Fill{{
			Path: tx.Child("montant"), Value: "250",
			Sources:    []core.SourceRef{{Object: core.ObjectRef{Table: "transactions", ObjectID: "t1"}, Field: "amount", RawValue: "250"}},
			Confidence: core.ConfidenceHigh,
		}},
	}})
	st, err = uc.ChooseQuestionOption(f.Ctx, q.Id, st.Declaration.Version, 0)
	require.NoError(t, err)

	line := compOnlyLine(t, st.Doc)
	assert.InDelta(t, 250, compDecimalOf(t, st.Doc.Values[line.Child("montantTotal")].Value), 0.001)
	assert.InDelta(t, 250, compDecimalOf(t, st.Doc.Values["operations.montantTotal"].Value), 0.001)
}

func TestTransactionWithoutCurrencyBlocksTheExport(t *testing.T) {
	uc, f := newTestUsecase(t)
	st := createFresh(t, uc, f)
	id := st.Declaration.Id

	tx, st, err := uc.AddItem(f.Ctx, id, st.Declaration.Version, "work.transactions")
	require.NoError(t, err)
	st, err = uc.UpdateFields(f.Ctx, id, st.Declaration.Version, []core.Fill{
		{Path: tx.Child("montant"), Value: "100.00"},
		{Path: tx.Child("date"), Value: "2026-03-10"},
	})
	require.NoError(t, err)

	var found bool
	for _, is := range st.Issues {
		if is.Code == "transaction_non_comptee" && is.Path == tx.Child("devise") && is.Blocks() {
			found = true
		}
	}
	require.True(t, found, "the uncounted transaction is a blocking issue on its currency field")
	_, err = uc.Export(f.Ctx, id, st.Declaration.Version)
	require.ErrorIs(t, err, models.BadParameterError)
}
