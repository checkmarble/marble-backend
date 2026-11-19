package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// An auto-added object is a selected object like the others for the sources a
// fill may cite, and the structured context reports it as auto-added.
func TestAutoAddedObjectIsCitableAndReported(t *testing.T) {
	txn := agentSelection("transactions", "txn-1", map[string]any{"object_id": "txn-1", "account_id": "acc-1"})
	acc := agentSelection("accounts", "acc-1", map[string]any{"object_id": "acc-1", "name": "ACME"})
	acc.AutoAdded, acc.ContextOnly = true, true
	sel := []models.SarSelectedObject{txn, acc}

	doc := core.NewDocument()
	item := doc.AddItem("work.transactions", "transactions")
	doc.ItemSources[item] = []core.ObjectRef{{Table: "transactions", ObjectID: "txn-1"}}
	allowed := AllowedSources(doc, sel, toolsDataModel(), nil)(item.Child("montant"))
	assert.Contains(t, allowed, core.ObjectRef{Table: "accounts", ObjectID: "acc-1"})

	sc := StructuredContextFor(sel, toolsDataModel())
	assert.True(t, sc.IsContextOnly(core.ObjectRef{Table: "accounts", ObjectID: "acc-1"}))
	assert.False(t, sc.IsContextOnly(core.ObjectRef{Table: "transactions", ObjectID: "txn-1"}))
}
