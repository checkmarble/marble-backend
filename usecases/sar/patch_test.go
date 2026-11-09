package sar

import (
	"encoding/json"
	"math/big"
	"strings"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sar/sartest"
	"github.com/stretchr/testify/require"
)

func newRat(t *testing.T, s string) (*big.Rat, bool) {
	t.Helper()
	return new(big.Rat).SetString(s)
}

func patchOp(t *testing.T, op, path string, value any) models.SarPatchOp {
	t.Helper()
	var raw json.RawMessage
	if value != nil {
		b, err := json.Marshal(value)
		require.NoError(t, err)
		raw = b
	}
	return models.SarPatchOp{Op: op, Path: path, Value: raw}
}

func patchCode(t *testing.T, err error) string {
	t.Helper()
	require.Error(t, err)
	var pe PatchError
	require.ErrorAs(t, err, &pe)
	return pe.Code
}

func TestApplyOpsManualValuesAreProtected(t *testing.T) {
	p := sartest.Pivot()
	p.Parties[0].Occupation = &models.SarText{V: "Consultant", P: &models.SarProvenance{Kind: models.SarProvManual, ActorID: "user-1"}}
	prov := models.SarProvenance{Kind: models.SarProvAI, Explanation: "simulé"}
	ops := []models.SarPatchOp{patchOp(t, "set", "pivot/parties/p1/occupation", "Retraité")}

	_, err := ApplyOps(p, Exts{}, ops, prov, Policy{})
	require.Equal(t, "PATCH_MANUAL_PROTECTED", patchCode(t, err))
	require.Equal(t, "Consultant", p.Parties[0].Occupation.V, "a failed patch must not change the pivot")

	touched, err := ApplyOps(p, Exts{}, ops, prov, Policy{AllowManualOverwrite: true})
	require.NoError(t, err)
	require.Equal(t, []string{"pivot/parties/p1/occupation"}, touched)
	require.Equal(t, "Retraité", p.Parties[0].Occupation.V)
	require.Equal(t, models.SarProvAI, p.Parties[0].Occupation.P.Kind)
}

func TestApplyOpsAnalysisIsProtected(t *testing.T) {
	p := sartest.Pivot()
	prov := models.SarProvenance{Kind: models.SarProvAI}
	ops := []models.SarPatchOp{patchOp(t, "set", "pivot/suspicion/analysis", "Nouvelle analyse")}

	_, err := ApplyOps(p, Exts{}, ops, prov, Policy{})
	require.Equal(t, "PATCH_PROTECTED", patchCode(t, err))

	_, err = ApplyOps(p, Exts{}, ops, prov, Policy{AllowManualOverwrite: true})
	require.Equal(t, "PATCH_PROTECTED", patchCode(t, err), "manual overwrite alone does not unlock the analysis")
	require.NotEqual(t, "Nouvelle analyse", p.Suspicion.Analysis.V)

	_, err = ApplyOps(p, Exts{}, ops, prov, Policy{AllowManualOverwrite: true, AllowAnalysis: true})
	require.NoError(t, err)
	require.Equal(t, "Nouvelle analyse", p.Suspicion.Analysis.V)
}

func TestApplyOpsPrefixesRestrictScope(t *testing.T) {
	p := sartest.Pivot()
	prov := models.SarProvenance{Kind: models.SarProvAI}
	policy := Policy{Prefixes: []string{"pivot/parties"}}

	_, err := ApplyOps(p, Exts{}, []models.SarPatchOp{patchOp(t, "set", "pivot/suspicion/actions_taken", "X")}, prov, policy)
	require.Equal(t, "PATCH_SCOPE", patchCode(t, err))

	_, err = ApplyOps(p, Exts{}, []models.SarPatchOp{patchOp(t, "set", "pivot/parties/p1/occupation", "Chef")}, prov, policy)
	require.NoError(t, err)
	require.Equal(t, "Chef", p.Parties[0].Occupation.V)
}

func TestApplyOpsAddObjectWrapsPlainValues(t *testing.T) {
	p := sartest.Pivot()
	prov := models.SarProvenance{Kind: models.SarProvManual, ActorID: "user-1"}
	value := map[string]any{
		"first_name": "Paul",
		"last_name":  "Durand",
		"roles":      []string{"counterparty"},
		"addresses": []any{
			map[string]any{"type": "business", "city": "Metz", "country": "FR"},
		},
	}

	_, err := ApplyOps(p, Exts{}, []models.SarPatchOp{patchOp(t, "add", "pivot/parties", value)}, prov, Policy{})
	require.NoError(t, err)
	require.Len(t, p.Parties, 4)

	np := p.Parties[3]
	require.NotEmpty(t, np.ID)
	require.True(t, strings.HasPrefix(np.ID, "p"), "party ID prefix, got %q", np.ID)

	require.Equal(t, "Paul", np.FirstName.V)
	require.Equal(t, models.SarProvManual, np.FirstName.P.Kind)
	require.Equal(t, "user-1", np.FirstName.P.ActorID)

	require.Len(t, np.Roles, 1)
	require.Equal(t, "counterparty", np.Roles[0].V)
	require.Equal(t, models.SarProvManual, np.Roles[0].P.Kind)

	require.Len(t, np.Addresses, 1)
	require.NotEmpty(t, np.Addresses[0].ID)
	require.Equal(t, "Metz", np.Addresses[0].City.V)
	require.Equal(t, "FR", np.Addresses[0].Country.V)
	require.Equal(t, models.SarProvManual, np.Addresses[0].City.P.Kind)
}

func TestApplyOpsSetListField(t *testing.T) {
	p := sartest.Pivot()
	prov := models.SarProvenance{Kind: models.SarProvManual, ActorID: "user-1"}

	_, err := ApplyOps(p, Exts{}, []models.SarPatchOp{patchOp(t, "set", "pivot/parties/p1/nationalities", []string{"FR", "LU"})}, prov, Policy{})
	require.NoError(t, err)

	nat := p.Parties[0].Nationalities
	require.Len(t, nat, 2)
	require.Equal(t, "FR", nat[0].V)
	require.Equal(t, "LU", nat[1].V)
	require.Equal(t, models.SarProvManual, nat[1].P.Kind)
}

func TestApplyOpsRemoveOperationByID(t *testing.T) {
	p := sartest.Pivot()
	prov := models.SarProvenance{Kind: models.SarProvManual}

	_, err := ApplyOps(p, Exts{}, []models.SarPatchOp{patchOp(t, "remove", "pivot/operations/op2", nil)}, prov, Policy{})
	require.NoError(t, err)
	require.Len(t, p.Operations, 2)
	for _, op := range p.Operations {
		require.NotEqual(t, "op2", op.ID)
	}
	require.Equal(t, "op1", p.Operations[0].ID)
	require.Equal(t, "op3", p.Operations[1].ID)
}

func TestApplyOpsRejectsUnknownRoot(t *testing.T) {
	p := sartest.Pivot()
	_, err := ApplyOps(p, Exts{}, []models.SarPatchOp{patchOp(t, "set", "parties/p1/occupation", "X")}, models.SarProvenance{Kind: models.SarProvManual}, Policy{})
	require.Equal(t, "PATCH_PATH", patchCode(t, err))
}

func TestConvert(t *testing.T) {
	rates := sartest.Rates()

	// same currency: rounded half-up, no conversion explanation
	v, why, ok := Convert("100.005", "EUR", "EUR", rates)
	require.True(t, ok)
	require.Equal(t, "100.01", v)
	require.Empty(t, why)

	// direct rate
	v, why, ok = Convert("100", "USD", "EUR", rates)
	require.True(t, ok)
	require.Equal(t, "92.00", v)
	require.Contains(t, why, "USD/EUR")

	// inverse rate: EUR -> USD uses 1 / 0.92
	v, _, ok = Convert("92", "EUR", "USD", rates)
	require.True(t, ok)
	require.Equal(t, "100.00", v)

	// half-up on the converted amount: 1 * 0.125 = 0.125 -> 0.13
	v, _, ok = Convert("1", "USD", "EUR", map[string]string{"USD/EUR": "0.125"})
	require.True(t, ok)
	require.Equal(t, "0.13", v)

	// missing rate: never inferred
	_, _, ok = Convert("1", "USD", "CHF", map[string]string{"EUR/CHF": "0.94"})
	require.False(t, ok)

	// zero inverse rate cannot be inverted
	_, _, ok = Convert("1", "EUR", "USD", map[string]string{"USD/EUR": "0"})
	require.False(t, ok)

	// invalid amount
	_, _, ok = Convert("12,5", "USD", "EUR", rates)
	require.False(t, ok)
}

func TestRoundHalfUp(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"2.345", 2, "2.35"},
		{"2.344", 2, "2.34"},
		{"-2.345", 2, "-2.35"},
		{"-0.001", 2, "0.00"},
		{"7", 0, "7"},
		{"0.5", 0, "1"},
		{"0.0004", 3, "0.000"},
	}
	for _, c := range cases {
		r, ok := newRat(t, c.in)
		require.True(t, ok)
		require.Equal(t, c.want, Round(r, c.n), "Round(%s, %d)", c.in, c.n)
	}
}

func TestSum(t *testing.T) {
	v, ok := Sum([]string{"0.1", "0.2"})
	require.True(t, ok)
	require.Equal(t, "0.30", v)

	v, ok = Sum([]string{"1.005", "1.005"})
	require.True(t, ok)
	require.Equal(t, "2.01", v)

	v, ok = Sum([]string{"-10", "2.5"})
	require.True(t, ok)
	require.Equal(t, "-7.50", v)

	_, ok = Sum([]string{"1", "abc"})
	require.False(t, ok)
}

func TestPattern(t *testing.T) {
	cases := map[string]string{
		"pivot/parties/p1/addresses/a1/city": "parties/*/addresses/*/city",
		"pivot/operations/op1/amount":        "operations/*/amount",
		"pivot/suspicion/analysis":           "suspicion/analysis",
		"pivot/parties/p1/nationalities/0":   "parties/*/nationalities",
		"pivot/parties/p1/nationalities":     "parties/*/nationalities",
	}
	for in, want := range cases {
		require.Equal(t, want, Pattern(in), in)
	}
}

func TestNormalizePivotPath(t *testing.T) {
	require.Equal(t, "pivot/operations/*/amount", NormalizePivotPath("pivot/operations/op1/amount"))
	require.Equal(t, "pivot/parties/*/addresses/*/city", NormalizePivotPath("pivot/parties/p1/addresses/a1/city"))
	require.Equal(t, "/report/transaction/amount", NormalizePivotPath("/report/transaction[3]/amount"))
	require.Equal(t, "pivot/parties/*/first_name", NormalizePivotPath("pivot/parties/p1/first_name#p1"))
}

func TestApplyOpsAddListValueOnListField(t *testing.T) {
	p := sartest.Pivot()
	_, err := ApplyOps(p, Exts{}, []models.SarPatchOp{patchOp(t, "add", "pivot/parties/p1/nationalities", []string{"LU", "DE"})}, models.SarProvenance{Kind: models.SarProvManual}, Policy{})
	if err == nil {
		for _, n := range p.Parties[0].Nationalities {
			require.NotEmpty(t, n.V, "no empty element may be appended")
		}
	}
}
