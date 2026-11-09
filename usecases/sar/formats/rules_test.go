package formats_test

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sar"
	"github.com/checkmarble/marble-backend/usecases/sar/format"
	"github.com/checkmarble/marble-backend/usecases/sar/formats/goaml"
	"github.com/checkmarble/marble-backend/usecases/sar/formats/tracfin"
	"github.com/checkmarble/marble-backend/usecases/sar/sartest"
	"github.com/stretchr/testify/require"
)

func extPatch(t *testing.T, op, path string, value any) models.SarPatchOp {
	t.Helper()
	var raw json.RawMessage
	if value != nil {
		b, err := json.Marshal(value)
		require.NoError(t, err)
		raw = b
	}
	return models.SarPatchOp{Op: op, Path: path, Value: raw}
}

func patchCodeOf(t *testing.T, err error) string {
	t.Helper()
	require.Error(t, err)
	var pe sar.PatchError
	require.ErrorAs(t, err, &pe)
	return pe.Code
}

func issuesWith(issues []models.SarIssue, code string) []models.SarIssue {
	var out []models.SarIssue
	for _, i := range issues {
		if i.Code == code {
			out = append(out, i)
		}
	}
	return out
}

func hasCode(issues []models.SarIssue, code string) bool {
	return len(issuesWith(issues, code)) > 0
}

// aiExt returns a fresh AI extension of one format.
func aiExt(key string) models.SarExtension { return sartest.AIExtension()[key] }

func aiValue(values ...string) models.SarExtValue {
	return models.SarExtValue{Values: values, P: &models.SarProvenance{Kind: models.SarProvAI}}
}

func TestExtensionOpsThroughApplyOps(t *testing.T) {
	const key = goaml.KeyLU
	prov := models.SarProvenance{Kind: models.SarProvAI, Explanation: "simulé"}

	t.Run("list field is set", func(t *testing.T) {
		p := sartest.Pivot()
		exts := sar.Exts{key: aiExt(key)}
		ops := []models.SarPatchOp{extPatch(t, "set", "extension/"+key+"/indicators", []string{"002TS-TxnPattern", "005PU-BankAcct"})}
		touched, err := sar.ApplyOps(p, exts, ops, prov, sar.Policy{})
		require.NoError(t, err)
		require.Equal(t, []string{"extension/" + key + "/indicators"}, touched)
		got := exts[key]["indicators"]
		require.Equal(t, []string{"002TS-TxnPattern", "005PU-BankAcct"}, got.Values)
		require.Equal(t, models.SarProvAI, got.P.Kind)
	})

	t.Run("computed field needs manual overwrite", func(t *testing.T) {
		p := sartest.Pivot()
		exts := sar.Exts{key: aiExt(key)}
		ops := []models.SarPatchOp{extPatch(t, "set", "extension/"+key+"/report_type", "SAR")}
		_, err := sar.ApplyOps(p, exts, ops, prov, sar.Policy{})
		require.Equal(t, "PATCH_COMPUTED", patchCodeOf(t, err))
		require.Empty(t, exts[key]["report_type"].Values)

		_, err = sar.ApplyOps(p, exts, ops, prov, sar.Policy{AllowManualOverwrite: true})
		require.NoError(t, err)
		require.Equal(t, []string{"SAR"}, exts[key]["report_type"].Values)
	})

	t.Run("settings cannot be patched", func(t *testing.T) {
		p := sartest.Pivot()
		exts := sar.Exts{key: aiExt(key)}
		ops := []models.SarPatchOp{extPatch(t, "set", "extension/"+key+"/rentity_id", "9999")}
		_, err := sar.ApplyOps(p, exts, ops, prov, sar.Policy{AllowManualOverwrite: true})
		require.Equal(t, "PATCH_SETTINGS", patchCodeOf(t, err))
	})

	t.Run("manual extension value is protected", func(t *testing.T) {
		p := sartest.Pivot()
		exts := sar.Exts{key: aiExt(key)}
		exts[key]["indicators"] = models.SarExtValue{Values: []string{"007SO-MoneyLaundering"}, P: &models.SarProvenance{Kind: models.SarProvManual, ActorID: "user-1"}}
		ops := []models.SarPatchOp{extPatch(t, "set", "extension/"+key+"/indicators", []string{"005PU-BankAcct"})}
		_, err := sar.ApplyOps(p, exts, ops, prov, sar.Policy{})
		require.Equal(t, "PATCH_MANUAL_PROTECTED", patchCodeOf(t, err))
		require.Equal(t, []string{"007SO-MoneyLaundering"}, exts[key]["indicators"].Values)
	})

	t.Run("unknown format and unknown field are rejected", func(t *testing.T) {
		p := sartest.Pivot()
		exts := sar.Exts{}
		_, err := sar.ApplyOps(p, exts, []models.SarPatchOp{extPatch(t, "set", "extension/goaml:XX:1/indicators", "x")}, prov, sar.Policy{})
		require.Equal(t, "PATCH_FORMAT", patchCodeOf(t, err))
		_, err = sar.ApplyOps(p, exts, []models.SarPatchOp{extPatch(t, "set", "extension/"+key+"/unknown_field", "x")}, prov, sar.Policy{})
		require.Equal(t, "PATCH_FIELD", patchCodeOf(t, err))
	})
}

func TestGoAMLLUTransferWithoutCounterpartyAccountRejTRS03(t *testing.T) {
	a, ok := sar.Get(goaml.KeyLU)
	require.True(t, ok)

	baseline := sar.Evaluate(a, input(goaml.KeyLU))
	require.Empty(t, issuesWith(baseline.Issues, "REJ-TRS03"))

	// op1 is a transfer (funds Q) to an entity without account: the credit side
	// carries no account, so the rule must fire on the transaction.
	in := input(goaml.KeyLU)
	in.Pivot.Operations[0].CounterpartyInstrument = nil
	ev := sar.Evaluate(a, in)
	found := issuesWith(ev.Issues, "REJ-TRS03")
	require.NotEmpty(t, found)
	require.True(t, strings.HasPrefix(found[0].Path, "/report/transaction["), found[0].Path)
}

func TestGoAMLLUSplitsAt500Transactions(t *testing.T) {
	a, ok := sar.Get(goaml.KeyLU)
	require.True(t, ok)

	in := input(goaml.KeyLU)
	base := in.Pivot.Operations
	ops := make([]models.SarOperation, 0, 501)
	for i := 0; i < 501; i++ {
		op := base[i%len(base)]
		op.ID = fmt.Sprintf("op%04d", i+1)
		op.Reference = &models.SarText{V: fmt.Sprintf("TRX-%05d", i+1), P: base[i%len(base)].Reference.P}
		ops = append(ops, op)
	}
	in.Pivot.Operations = ops

	ev := sar.Evaluate(a, in)
	require.Len(t, ev.Docs, 2)
	require.Empty(t, issuesWith(ev.Issues, "REJ-TRS02"))
	require.Empty(t, issuesWith(ev.Issues, "REJ-TRS01"))
	require.True(t, strings.HasSuffix(ev.Docs[0].Str("entity_reference"), "-1"), ev.Docs[0].Str("entity_reference"))
	require.True(t, strings.HasSuffix(ev.Docs[1].Str("entity_reference"), "-2"), ev.Docs[1].Str("entity_reference"))
	require.Len(t, ev.Docs[0].All("transaction"), 500)
	require.Len(t, ev.Docs[1].All("transaction"), 1)

	for _, d := range ev.Docs {
		doc, err := format.XMLBytes(a.Schema(), d)
		require.NoError(t, err)
		sartest.XMLLint(t, goaml.KeyLU, doc)
	}
}

func TestGoAMLCHTwoMIndicatorsRejectMROSA05(t *testing.T) {
	a, ok := sar.Get(goaml.KeyCH)
	require.True(t, ok)

	baseline := sar.Evaluate(a, input(goaml.KeyCH))
	require.Empty(t, issuesWith(baseline.Issues, "MROS-A05"))

	in := input(goaml.KeyCH)
	in.Ext["indicators"] = aiValue("0002M", "0004M", "1004V", "2018G", "3007B")
	ev := sar.Evaluate(a, in)
	found := issuesWith(ev.Issues, "MROS-A05")
	require.NotEmpty(t, found)
	require.Equal(t, "/report/report_indicators[1]", found[0].Path)
}

func TestGoAMLCHAccountBalanceOlderThan14DaysRejectMROSA13(t *testing.T) {
	a, ok := sar.Get(goaml.KeyCH)
	require.True(t, ok)

	baseline := sar.Evaluate(a, input(goaml.KeyCH))
	require.Empty(t, issuesWith(baseline.Issues, "MROS-A13"))

	in := input(goaml.KeyCH)
	old := time.Now().UTC().AddDate(0, 0, -30).Format("2006-01-02")
	in.Pivot.Instruments[0].BalanceDate = &models.SarText{V: old}
	ev := sar.Evaluate(a, in)
	require.NotEmpty(t, issuesWith(ev.Issues, "MROS-A13"))
}

func TestTracfinOperationsAtMost30Lines(t *testing.T) {
	a, ok := sar.Get(tracfin.Key)
	require.True(t, ok)

	in := input(tracfin.Key)
	base := in.Pivot.Operations[0]
	var ops []models.SarOperation
	for i := 0; i < 40; i++ {
		cpID := fmt.Sprintf("cp%02d", i+1)
		in.Pivot.Parties = append(in.Pivot.Parties, models.SarParty{
			ID:        cpID,
			Kind:      &models.SarText{V: "entity"},
			Roles:     []models.SarText{{V: "counterparty"}},
			Name:      &models.SarText{V: fmt.Sprintf("Contrepartie %02d", i+1)},
			Addresses: []models.SarAddress{{ID: "ad" + cpID, Country: &models.SarText{V: "CY"}}},
		})
		op := base
		op.ID = fmt.Sprintf("op%02d", i+1)
		op.Reference = &models.SarText{V: fmt.Sprintf("TRX-%04d", i+1)}
		op.Timestamp = &models.SarText{V: fmt.Sprintf("2026-10-%02dT10:00:00+02:00", i%28+1)}
		op.CounterpartyID = &models.SarText{V: cpID}
		op.CounterpartyInstrument = nil
		if i%2 == 1 {
			op.Direction = &models.SarText{V: "incoming"}
		}
		ops = append(ops, op)
	}
	in.Pivot.Operations = ops

	ev := sar.Evaluate(a, in)
	require.Len(t, ev.Docs, 1)
	operations := ev.Docs[0].Get("operations")
	require.NotNil(t, operations)

	lines := operations.All("operation")
	require.NotEmpty(t, lines)
	require.LessOrEqual(t, len(lines), 30)

	total := 0
	for _, l := range lines {
		require.Contains(t, l.Str("commentaires"), "regroupement")
		n, err := strconv.Atoi(l.Str("nombreOperations"))
		require.NoError(t, err)
		total += n
	}
	require.Equal(t, 40, total, "every unit operation is counted once")
}

func TestTracfinLegalGroundRequired(t *testing.T) {
	a, ok := sar.Get(tracfin.Key)
	require.True(t, ok)

	in := input(tracfin.Key)
	delete(in.Ext, "cadre_legal")
	ev := sar.Evaluate(a, in)
	require.True(t, hasCode(ev.Issues, "EXT_REQUIRED") || hasCode(ev.Issues, "TF-L1685-LEGAL_GROUND"), "%v", ev.Issues)
}

func TestTracfinTaxFraudNatureRequiredWithCL03(t *testing.T) {
	a, ok := sar.Get(tracfin.Key)
	require.True(t, ok)

	in := input(tracfin.Key)
	in.Ext["cadre_legal"] = aiValue("CL03")
	delete(in.Ext, "nature_soupcon_penale")
	delete(in.Ext, "nature_fraude_fiscale")
	ev := sar.Evaluate(a, in)
	require.True(t, hasCode(ev.Issues, "TF-L1709-TAX_REQUIRED"), "%v", ev.Issues)
}

func TestTracfinRepresentEditPaths(t *testing.T) {
	a, ok := sar.Get(tracfin.Key)
	require.True(t, ok)

	ev := sar.Evaluate(a, input(tracfin.Key))
	require.Empty(t, blocking(ev.Issues))
	require.Len(t, ev.Docs, 1)

	view := format.Represent(a.Schema(), ev.Docs[0], ev.Issues, a.Hint)

	var analyse *format.ViewNode
	leaves := 0
	var walk func(v *format.ViewNode)
	walk = func(v *format.ViewNode) {
		if v.Path == "/ds/analyse[1]" {
			analyse = v
		}
		if v.Value != nil && v.Edit != "" {
			leaves++
			ok := strings.HasPrefix(v.Edit, "pivot/") || strings.HasPrefix(v.Edit, "extension/") || strings.HasPrefix(v.Edit, "settings/")
			require.True(t, ok, "%s carries edit %q", v.Path, v.Edit)
		}
		for i := range v.Children {
			walk(&v.Children[i])
		}
	}
	walk(&view)

	require.NotNil(t, analyse, "analyse node in the view")
	require.Equal(t, "pivot/suspicion/analysis", analyse.Edit)
	require.Positive(t, leaves)
}

func TestGoAMLLUForeignExchangeRateKeepsPrecision(t *testing.T) {
	a, ok := sar.Get(goaml.KeyLU)
	require.True(t, ok)
	in := input(goaml.KeyLU)
	in.Rates = map[string]string{"EUR/USD": "1.087"}
	in.Pivot.Operations[0].Currency = &models.SarText{V: "USD"}
	ev := sar.Evaluate(a, in)
	var rates []string
	for _, d := range ev.Docs {
		collectText(d, "foreign_exchange_rate", &rates)
	}
	require.NotEmpty(t, rates)
	for _, r := range rates {
		require.True(t, strings.HasPrefix(r, "0.9199"), "rate %s, expected 1/1.087 with its precision", r)
	}
}

func collectText(n *format.Node, name string, out *[]string) {
	if n.Name == name && n.Text != nil {
		*out = append(*out, *n.Text)
	}
	for _, c := range n.Children {
		collectText(c, name, out)
	}
}
