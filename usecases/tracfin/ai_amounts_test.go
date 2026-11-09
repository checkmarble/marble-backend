package tracfin

import (
	"encoding/json"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/invopop/jsonschema"
	"github.com/stretchr/testify/require"
)

func TestResolveAIAmountsExact(t *testing.T) {
	sources := []models.TracfinAISource{{ID: "decision:1", Content: json.RawMessage(`{"amount":125000,"large":9007199254740993,"currency":"EUR","minor_unit":"minor","major_unit":"major","fractional":1.5,"jpy":"JPY"}`)}}
	for _, tc := range []struct{ locator, unit, want string }{{"/amount", "minor", "1250.00"}, {"/large", "major", "9007199254740993"}} {
		p := AISectionProposal{Values: []AIValue{{Kind: "decimal", Explanation: "source", Calculation: &AICalculation{SourceID: "decision:1", Locator: tc.locator, Unit: tc.unit, UnitSourceID: "decision:1", UnitLocator: "/" + tc.unit + "_unit", CurrencySourceID: "decision:1", CurrencyLocator: "/currency", Currency: "EUR"}}}}
		ds := ResolveAIAmounts(&p, sources)
		require.Empty(t, ds)
		require.Equal(t, tc.want, string(*p.Values[0].Decimal))
		require.Contains(t, p.Values[0].Explanation, "EUR")
		require.Len(t, p.Values[0].Sources, 3)
	}
}

func TestResolveAIAmountsRejectsUnproven(t *testing.T) {
	sources := []models.TracfinAISource{{ID: "decision:1", Content: json.RawMessage(`{"amount":125000,"currency":"EUR","minor_unit":"minor","major_unit":"major","fractional":1.5,"jpy":"JPY"}`)}}
	valid := AICalculation{SourceID: "decision:1", Locator: "/amount", Unit: "minor", UnitSourceID: "decision:1", UnitLocator: "/minor_unit", CurrencySourceID: "decision:1", CurrencyLocator: "/currency", Currency: "EUR"}
	for _, tc := range []struct {
		name string
		calc *AICalculation
	}{
		{"no calculation", nil},
		{"missing unit proof", func() *AICalculation { x := valid; x.UnitLocator = ""; return &x }()},
		{"mismatched unit proof", func() *AICalculation { x := valid; x.UnitLocator = "/major_unit"; return &x }()},
		{"fractional minor unit", func() *AICalculation { x := valid; x.Locator = "/fractional"; return &x }()},
		{"non-EUR minor unit", func() *AICalculation { x := valid; x.Currency = "JPY"; x.CurrencyLocator = "/jpy"; return &x }()},
		{"invalid unit", func() *AICalculation { x := valid; x.Unit = "cents-ish"; return &x }()},
		{"currency mismatch", func() *AICalculation { x := valid; x.Currency = "USD"; return &x }()},
		{"bad amount pointer", func() *AICalculation { x := valid; x.Locator = "/missing"; return &x }()},
		{"bad currency pointer", func() *AICalculation { x := valid; x.CurrencyLocator = "/missing"; return &x }()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := AISectionProposal{Values: []AIValue{{Kind: "decimal", Calculation: tc.calc}}}
			require.NotEmpty(t, ResolveAIAmounts(&p, sources))
			require.Nil(t, p.Values[0].Decimal)
		})
	}
}

func TestResolveAIAmountsSum(t *testing.T) {
	sources := []models.TracfinAISource{{ID: "decision:1", Content: json.RawMessage(`{"first":125000,"second":500,"minor":"minor","major":"major","currency":"EUR","usd":"USD"}`)}}
	operand := func(path, unit string) AICalculationOperand {
		return AICalculationOperand{SourceID: "decision:1", Locator: path, Unit: unit, UnitSourceID: "decision:1", UnitLocator: "/" + unit, CurrencySourceID: "decision:1", CurrencyLocator: "/currency", Currency: "EUR"}
	}
	first, second := operand("/first", "minor"), operand("/second", "major")
	p := AISectionProposal{Values: []AIValue{{Kind: "decimal", Calculation: &AICalculation{Operation: "sum", Operands: []AICalculationOperand{first, second}}}}}
	require.Empty(t, ResolveAIAmounts(&p, sources))
	require.Equal(t, "1750.00", string(*p.Values[0].Decimal))
	require.Len(t, p.Values[0].Sources, 6)
	require.Contains(t, p.Values[0].Explanation, "125000")
	require.Contains(t, p.Values[0].Explanation, "500")
	second.Currency = "USD"
	second.CurrencyLocator = "/usd"
	p.Values[0].Calculation.Operands[1] = second
	require.NotEmpty(t, ResolveAIAmounts(&p, sources))
	require.Nil(t, p.Values[0].Decimal)
}

func TestValidateAIAmountTotalsUsesExactAcceptedOperations(t *testing.T) {
	amount := func(s string) models.TracfinValue {
		v := models.TracfinDecimal(s)
		return models.TracfinValue{Kind: "decimal", Decimal: &v}
	}
	total := "/ds/operations/montantTotal"
	part := "/ds/operations/operation/montantTotal"
	d := models.TracfinDraft{Configuration: models.TracfinConfiguration{Mappings: []models.TracfinMapping{{TargetField: total, TargetCurrency: "EUR"}, {TargetField: part, TargetCurrency: "EUR"}}}, Document: models.TracfinDocument{Root: models.TracfinBlock{CatalogID: "/ds/operations", Fields: map[string][]models.TracfinValue{total: {amount("9007199254740995")}}, Children: []models.TracfinBlock{{CatalogID: "/ds/operations/operation", Fields: map[string][]models.TracfinValue{part: {amount("9007199254740993")}}}, {CatalogID: "/ds/operations/operation", Fields: map[string][]models.TracfinValue{part: {amount("2")}}}}}}}
	require.Empty(t, ValidateAIAmountTotals(d, Catalog{}))
	d.Document.Root.Fields[total] = []models.TracfinValue{amount("9007199254740994")}
	require.Equal(t, "AMOUNT_TOTAL_MISMATCH", ValidateAIAmountTotals(d, Catalog{})[0].Code)
	d.Configuration.Mappings = nil
	require.Equal(t, "AMOUNT_TOTAL_CURRENCY_UNPROVEN", ValidateAIAmountTotals(d, Catalog{})[0].Code)
}

func sumFixture() ([]models.TracfinAISource, func(string) AICalculationOperand) {
	sources := []models.TracfinAISource{{ID: "decision:1", Content: json.RawMessage(`{"first":9900,"second":9900,"unit":"major","currency":"EUR","usd":"USD"}`)}}
	return sources, func(path string) AICalculationOperand {
		return AICalculationOperand{SourceID: "decision:1", Locator: path, Unit: "major", UnitSourceID: "decision:1", UnitLocator: "/unit", CurrencySourceID: "decision:1", CurrencyLocator: "/currency", Currency: "EUR"}
	}
}

// The response schema requires unit/currency members on every calculation, so a
// model fills them on a sum too: they are redundant with the operands and ignored.
func TestResolveAIAmountsSumIgnoresSchemaFilledScalarMembers(t *testing.T) {
	sources, operand := sumFixture()
	c := &AICalculation{Operation: "sum", Operands: []AICalculationOperand{operand("/first"), operand("/second")},
		SourceID: "decision:1", Locator: "/first", Unit: "major", UnitSourceID: "decision:1", UnitLocator: "/unit", CurrencySourceID: "decision:1", CurrencyLocator: "/currency", Currency: "EUR"}
	p := AISectionProposal{Values: []AIValue{{Kind: "decimal", Calculation: c}}}
	require.Empty(t, ResolveAIAmounts(&p, sources))
	require.Equal(t, "19800", string(*p.Values[0].Decimal))
}

func TestResolveAIAmountsSumErrorsExplainTheFix(t *testing.T) {
	sources, operand := sumFixture()
	bad := operand("/missing")
	usd := operand("/second")
	usd.Currency, usd.CurrencyLocator = "USD", "/usd"
	for _, tc := range []struct {
		name string
		calc AICalculation
		want []string
	}{
		{"no operands", AICalculation{Operation: "sum", Unit: "major", Currency: "EUR"}, []string{"calculation.operands", "source_id", "currency_locator"}},
		{"operands without sum", AICalculation{Operands: []AICalculationOperand{operand("/first")}}, []string{`operation must be "sum"`}},
		{"invalid operand", AICalculation{Operation: "sum", Operands: []AICalculationOperand{operand("/first"), bad}}, []string{"operand 2", "JSON pointer"}},
		{"mixed currencies", AICalculation{Operation: "sum", Operands: []AICalculationOperand{operand("/first"), usd}}, []string{"operand 2", "EUR", "USD"}},
		{"contradicting sum currency", AICalculation{Operation: "sum", Operands: []AICalculationOperand{operand("/first")}, Currency: "USD"}, []string{"USD", "EUR"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.calc
			p := AISectionProposal{Values: []AIValue{{Kind: "decimal", Calculation: &c}}}
			ds := ResolveAIAmounts(&p, sources)
			require.Len(t, ds, 1)
			for _, w := range tc.want {
				require.Contains(t, ds[0].Message, w)
			}
			require.Nil(t, p.Values[0].Decimal)
		})
	}
}

func TestResolveAIAmountsRejectsModelDecimalWithTheFix(t *testing.T) {
	sources, _ := sumFixture()
	d := models.TracfinDecimal("9900")
	p := AISectionProposal{Values: []AIValue{{Key: "amount_3", FieldID: "/ds/operations/operation/montantTotal", Kind: "decimal", Decimal: &d}}}
	ds := ResolveAIAmounts(&p, sources)
	require.Len(t, ds, 1)
	for _, w := range []string{"amount_3", "do not set decimal", "currency_locator"} {
		require.Contains(t, ds[0].Message, w)
	}
	require.Nil(t, p.Values[0].Decimal)
}

func TestAIProposalSchemaHidesComputedDecimal(t *testing.T) {
	raw, err := json.Marshal(jsonschema.Reflect(AISectionProposal{}))
	require.NoError(t, err)
	require.NotContains(t, string(raw), `"decimal":`)
}
