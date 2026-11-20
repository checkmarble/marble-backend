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

func TestStructuredContextCarriesTableSemantics(t *testing.T) {
	dm := toolsDataModel()
	tr := dm.Tables["transactions"]
	tr.SemanticType = models.SemanticTypeTransaction
	dm.Tables["transactions"] = tr
	ac := dm.Tables["accounts"]
	ac.SemanticType = models.SemanticTypeAccount
	dm.Tables["accounts"] = ac
	sel := []models.SarSelectedObject{
		agentSelection("transactions", "txn-1", map[string]any{"object_id": "txn-1"}),
		agentSelection("accounts", "acc-1", map[string]any{"object_id": "acc-1"}),
	}
	sc := StructuredContextFor(sel, dm)
	assert.False(t, sc.CanCarryEntity(core.ObjectRef{Table: "transactions", ObjectID: "txn-1"}))
	assert.False(t, sc.CanCarryEntity(core.ObjectRef{Table: "accounts", ObjectID: "acc-1"}))
	assert.True(t, sc.CanCarryEntity(core.ObjectRef{Table: "unknown", ObjectID: "x"}), "unknown semantic: nothing assumed")
}

// The structured context carries, for each auto-added object, the object it was
// reached from, so that a format can follow the chain up to the analyst's choice.
func TestStructuredContextCarriesTheReachedFromChain(t *testing.T) {
	txn := agentSelection("transactions", "txn-1", map[string]any{"object_id": "txn-1"})
	acc := agentSelection("accounts", "acc-1", map[string]any{"object_id": "acc-1"})
	acc.AutoAdded, acc.ContextOnly = true, true
	acc.ReachedFrom = &models.SarReachedFrom{TableName: "transactions", ObjectId: "txn-1", Link: "account"}
	holder := agentSelection("persons", "p-1", map[string]any{"object_id": "p-1"})
	holder.AutoAdded, holder.ContextOnly = true, true
	holder.ReachedFrom = &models.SarReachedFrom{TableName: "accounts", ObjectId: "acc-1", Link: "holder"}

	sc := StructuredContextFor([]models.SarSelectedObject{txn, acc, holder}, toolsDataModel())

	choice, _, ok := sc.ChoiceBehind(core.ObjectRef{Table: "persons", ObjectID: "p-1"})
	assert.True(t, ok)
	assert.Equal(t, core.ObjectRef{Table: "transactions", ObjectID: "txn-1"}, choice)
	_, known := sc.ReachedFrom[core.ObjectRef{Table: "transactions", ObjectID: "txn-1"}]
	assert.False(t, known, "an analyst object has no reached-from")
}
