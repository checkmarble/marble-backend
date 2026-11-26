package sargen

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/checkmarble/marble-backend/usecases/sargen/engine"
)

// T2 (PR6 proof item 5, 4.1): the placement lists each transaction WITHOUT its
// account. The backend adds the selected account linked to the transaction by
// the data model (transactions.account_id -> accounts.object_id), so the working
// transactions keep the account as a source and refEmission is derived from it.
//
// Expected new call: ApplyStructured(kind, doc, out, structuredContext) from
// the orchestrator, which already holds r.selection and r.dataModel.
func TestGenerate_PlacementWithoutAccountLinksItFromTheDataModel(t *testing.T) {
	uc, f, declId := e2eReady(t)
	selDataModel(f)

	obj := func(table, id string) m { return m{"table": table, "object_id": id} }
	txObjects := make([]m, 0, len(e2eTransactions))
	for _, tx := range e2eTransactions {
		txObjects = append(txObjects, m{"object": obj("transactions", tx.id)})
	}
	script := e2eScript()
	script["sar_generation/tracfin/placement.md"] = []engine.AgentStep{e2eFinal(m{
		"persons": []m{{"kind": "pp", "objects": []m{obj("clients", "c1")}}},
		"financial_refs": []m{{
			"ref_kind": "Iban", "objects": []m{obj("accounts", "a1")},
			"owners": []m{{"person_index": 0, "lien": "titulaire"}},
		}},
		"transactions": txObjects,
	})}
	llm := &e2eLLM{script: script, runs: map[string]int{}, Requests: map[string]int{}}
	o, _ := e2eOrchestrator(f, llm, e2ePrompts{})

	job, err := uc.StartGeneration(f.Ctx, declId)
	require.NoError(t, err)
	require.NoError(t, o.RunJob(f.Ctx, job.Id))

	done, err := f.Gen.GetSarGenerationJob(f.Ctx, nil, job.Id)
	require.NoError(t, err)
	require.Equal(t, models.SarJobStatusDone, done.Status, done.Error)

	st, err := uc.GetDeclaration(f.Ctx, declId)
	require.NoError(t, err)
	require.Len(t, st.Doc.ItemKeys("work.transactions"), len(e2eTransactions))
	for _, k := range st.Doc.ItemKeys("work.transactions") {
		tx := core.Path("work.transactions[" + k + "]")
		assert.Contains(t, st.Doc.ItemSources(tx), core.ObjectRef{Table: "accounts", ObjectID: "a1"},
			"the selected account linked to the transaction is one of its sources (%s)", tx)
		assert.Equal(t, e2eIbanItem, st.Doc.Values[tx.Child("refEmission")].Value,
			"refEmission is derived from the account the transaction shares with the reference (%s)", tx)
	}
}

// T5 (PR6 jobs item 5): the placement budget grows with the selection. Here 200
// transactions are selected and the placement reads 40 objects one by one before
// its final output: the default light budget (30 tool calls, 15 turns) ends the
// placement on a limit, so nothing is placed. The budget must cover the selection.
//
// The placement is the only step under test: later steps may fail on the scripted
// answers, which are not written for 200 transactions.
func TestGenerate_PlacementBudgetGrowsWithSelection(t *testing.T) {
	uc, f, declId := e2eReady(t)

	now := time.Now()
	bulk := make([]string, 0, 195)
	add := []SelectionAdd{}
	for i := 0; i < 195; i++ {
		id := fmt.Sprintf("bulk-%03d", i)
		bulk = append(bulk, id)
		f.Decisions.Add(f.OrgId, "transactions", map[string]any{
			"object_id": id, "amount": 1.0, "currency": "EUR", "date": "2026-03-01", "direction": "debit",
			"type_label": "virement", "account_id": "a1", "counterparty": "Société Beta SARL",
			"counterparty_iban": "FR7612345987650123456789014", "country_from": "FR", "country_to": "DE",
		}, now)
		add = append(add, SelectionAdd{Ref: selRef("transactions", id)})
	}
	current, err := uc.GetDeclaration(f.Ctx, declId)
	require.NoError(t, err)
	_, err = uc.UpdateSelection(f.Ctx, declId, current.Declaration.Version, add, nil)
	require.NoError(t, err)

	obj := func(table, id string) m { return m{"table": table, "object_id": id} }
	ids := []string{}
	for _, tx := range e2eTransactions {
		ids = append(ids, tx.id)
	}
	ids = append(ids, bulk...)
	txObjects := make([]m, 0, len(ids))
	for _, id := range ids {
		txObjects = append(txObjects, m{"object": obj("transactions", id)})
	}
	steps := make([]engine.AgentStep, 0, 41)
	for i := 0; i < 40; i++ {
		steps = append(steps, e2eCall("read_object", `{"table":"transactions","object_id":"t1"}`))
	}
	steps = append(steps, e2eFinal(m{
		"persons": []m{{"kind": "pp", "objects": []m{obj("clients", "c1")}}},
		"financial_refs": []m{{
			"ref_kind": "Iban", "objects": []m{obj("accounts", "a1")},
			"owners": []m{{"person_index": 0, "lien": "titulaire"}},
		}},
		"transactions": txObjects,
	}))
	script := e2eScript()
	script["sar_generation/tracfin/placement.md"] = steps
	llm := &e2eLLM{script: script, runs: map[string]int{}, Requests: map[string]int{}}
	o, repo := e2eOrchestrator(f, llm, e2ePrompts{})

	job, err := uc.StartGeneration(f.Ctx, declId)
	require.NoError(t, err)
	_ = o.RunJob(f.Ctx, job.Id)

	placements := repo.tasksOfKind("placement")
	require.Len(t, placements, 1)
	assert.Equal(t, models.SarJobStatusDone, placements[0].Status, placements[0].Error)

	st, err := uc.GetDeclaration(f.Ctx, declId)
	require.NoError(t, err)
	assert.Len(t, st.Doc.ItemKeys("work.transactions"), len(ids),
		"the placement of a 200-object selection completes within its budget")
}
