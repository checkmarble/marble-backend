package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

const txPaysOrigine = "work.transactions[transactions-1].paysOrigine"

func withTransaction(country string) func(d *core.Document) {
	return func(d *core.Document) {
		tx := d.AddItem("work.transactions", "transactions")
		for name, v := range map[string]string{"typeOperation": "OT01", "devise": "EUR", "montant": "100", "date": "2026-01-02"} {
			d.Values[tx.Child(name)] = core.Value{Value: v, Origin: core.OriginLLM}
		}
		if country != "" {
			d.Values[tx.Child("paysOrigine")] = core.Value{Value: country, Origin: core.OriginLLM}
		}
	}
}

// The origin country of a transaction, required by the format unless the
// operation is a cash deposit, gets the unknown-country question.
func TestFinish_EmptyOriginCountryOfATransactionOffersXX(t *testing.T) {
	f := finishJob(t, withTransaction(""))

	q, opts, ok := questionOn(t, f, txPaysOrigine)
	require.True(t, ok, "a question on the empty origin country")
	assert.Equal(t, models.SarQuestionKindQuestion, q.Kind)
	require.Len(t, opts, 1)
	assert.Equal(t, "Inconnu", opts[0].Label)
	assert.Equal(t, core.Path(txPaysOrigine), opts[0].Fills[0].Path)
	assert.Equal(t, "XX", opts[0].Fills[0].Value)
}

func TestFinish_FilledOriginCountryGetsNoQuestion(t *testing.T) {
	f := finishJob(t, withTransaction("FR"))
	_, _, ok := questionOn(t, f, txPaysOrigine)
	assert.False(t, ok)
}

// A cash deposit (OT07) does not need an origin country.
func TestFinish_CashDepositNeedsNoOriginCountry(t *testing.T) {
	f := finishJob(t, func(d *core.Document) {
		withTransaction("")(d)
		d.Values["work.transactions[transactions-1].typeOperation"] = core.Value{Value: "OT07", Origin: core.OriginLLM}
	})
	_, _, ok := questionOn(t, f, txPaysOrigine)
	assert.False(t, ok)
}
