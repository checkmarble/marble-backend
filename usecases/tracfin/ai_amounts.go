package tracfin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"

	"github.com/checkmarble/marble-backend/models"
)

// AICalculation cites the original amount and currency. The server computes the decimal.
type AICalculation struct {
	Operation        string                 `json:"operation,omitempty" jsonschema_description:"Empty for a copied amount (fill the scalar members). sum for a total: fill operands; the scalar members are then ignored."`
	Operands         []AICalculationOperand `json:"operands,omitempty" jsonschema_description:"Required when operation is sum: one entry per source amount to add (every declared principal operation), each with its own evidence. Never a precomputed subtotal."`
	SourceID         string                 `json:"source_id"`
	Locator          string                 `json:"locator"`
	Unit             string                 `json:"unit" jsonschema:"enum=major,enum=minor" jsonschema_description:"major for euros, minor for cents, as proven by unit_source_id/unit_locator."`
	UnitSourceID     string                 `json:"unit_source_id"`
	UnitLocator      string                 `json:"unit_locator"`
	CurrencySourceID string                 `json:"currency_source_id"`
	CurrencyLocator  string                 `json:"currency_locator"`
	Currency         string                 `json:"currency" jsonschema_description:"Explicit ISO 4217 three-letter code proven by currency_locator."`
}

// AICalculationOperand identifies each independently evidenced sum operand.
type AICalculationOperand struct {
	SourceID         string `json:"source_id" jsonschema_description:"Source holding this amount."`
	Locator          string `json:"locator" jsonschema_description:"JSON Pointer to the source number, for example /object/Data/amount."`
	Unit             string `json:"unit" jsonschema:"enum=major,enum=minor" jsonschema_description:"major for euros, minor for cents, as proven by unit_source_id/unit_locator."`
	UnitSourceID     string `json:"unit_source_id"`
	UnitLocator      string `json:"unit_locator" jsonschema_description:"JSON Pointer to a source value equal to unit."`
	CurrencySourceID string `json:"currency_source_id"`
	CurrencyLocator  string `json:"currency_locator" jsonschema_description:"JSON Pointer to the source currency."`
	Currency         string `json:"currency" jsonschema_description:"Explicit ISO 4217 three-letter code proven by currency_locator; the same for all operands."`
}

var aiDecimalPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)
var aiCurrencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

// ResolveAIAmounts replaces every model-supplied decimal with an exact calculation
// from an immutable source. It does not perform currency conversion.
func ResolveAIAmounts(p *AISectionProposal, sources []models.TracfinAISource) []models.TracfinDiagnostic {
	byID := make(map[string]json.RawMessage, len(sources))
	for _, s := range sources {
		byID[s.ID] = s.Content
	}
	var diagnostics []models.TracfinDiagnostic
	for i := range p.Values {
		value := &p.Values[i]
		if value.Kind != "decimal" && value.Decimal == nil && value.Calculation == nil {
			continue
		}
		c := value.Calculation
		provided := value.Decimal != nil
		value.Decimal = nil
		invalid := func(reason string) {
			diagnostics = append(diagnostics, aiDiagnostic("AI_AMOUNT_INVALID", "value "+value.Key+" ("+value.FieldID+"): "+reason, &models.TracfinFieldRef{FieldID: value.FieldID}))
		}
		if value.Kind != "decimal" || c == nil {
			reason := "a decimal value requires calculation"
			if provided {
				reason = "do not set decimal, the backend computes it; remove decimal and set calculation"
			}
			invalid(reason + `: {"source_id", "locator" (JSON Pointer to the source amount), "unit", "unit_source_id", "unit_locator", "currency_source_id", "currency_locator", "currency"}, or operation "sum" with operands for a total`)
			continue
		}
		if c.Operation != "" || len(c.Operands) != 0 {
			// The response schema makes the scalar members mandatory, so a sum may
			// carry them too. The operands are the evidence: these members are
			// ignored, except a currency contradicting the operands.
			if c.Operation != "sum" {
				invalid(`calculation.operation must be "sum" when calculation.operands is given, or empty with no operands for a copied amount`)
				continue
			}
			if len(c.Operands) == 0 || len(c.Operands) > 1000 {
				invalid(fmt.Sprintf("calculation.operation=sum requires calculation.operands with 1 to 1000 entries (got %d): one object per amount to add, each with source_id, locator, unit, unit_source_id, unit_locator, currency_source_id, currency_locator and currency", len(c.Operands)))
				continue
			}
			total := new(big.Rat)
			var evidence []AIEvidence
			var descriptions []string
			scale := 0
			currency := c.Operands[0].Currency
			problem := ""
			for i, operand := range c.Operands {
				if operand.Currency != currency {
					problem = fmt.Sprintf("operand %d has currency %q while operand 1 has %q; all operands must share one currency", i+1, operand.Currency, currency)
					break
				}
				input := AISectionProposal{Values: []AIValue{{Kind: "decimal", Calculation: &AICalculation{SourceID: operand.SourceID, Locator: operand.Locator, Unit: operand.Unit, UnitSourceID: operand.UnitSourceID, UnitLocator: operand.UnitLocator, CurrencySourceID: operand.CurrencySourceID, CurrencyLocator: operand.CurrencyLocator, Currency: operand.Currency}}}}
				if ds := ResolveAIAmounts(&input, sources); len(ds) != 0 {
					problem = fmt.Sprintf("operand %d (%s %s): %s", i+1, operand.SourceID, operand.Locator, ds[0].Message)
					break
				}
				result := input.Values[0]
				raw := string(*result.Decimal)
				number, ok := new(big.Rat).SetString(raw)
				if !ok {
					problem = fmt.Sprintf("operand %d is not an exact decimal", i+1)
					break
				}
				total.Add(total, number)
				if dot := strings.IndexByte(raw, '.'); dot >= 0 {
					scale = max(scale, len(raw)-dot-1)
				}
				evidence = append(evidence, result.Sources...)
				descriptions = append(descriptions, result.Explanation)
			}
			if problem == "" && c.Currency != "" && c.Currency != currency {
				problem = fmt.Sprintf("calculation.currency %q contradicts the operands currency %q", c.Currency, currency)
			}
			if problem != "" {
				invalid("Sum rejected: " + problem)
				continue
			}
			decimal := models.TracfinDecimal(total.FloatString(scale))
			value.Decimal = &decimal
			value.Sources = append(value.Sources, evidence...)
			value.Explanation = strings.TrimSpace(value.Explanation + " Backend sum: " + strings.Join(descriptions, " ") + fmt.Sprintf(" Total = %s %s.", decimal, currency))
			continue
		}
		amount, ok := aiJSONPointer(byID[c.SourceID], c.Locator)
		if !ok {
			invalid("Amount source or JSON pointer is invalid")
			continue
		}
		raw := ""
		switch n := amount.(type) {
		case json.Number:
			raw = n.String()
		case string:
			raw = n
		}
		if len(raw) > 100 || !aiDecimalPattern.MatchString(raw) {
			invalid("Amount must be a bounded decimal number")
			continue
		}
		if at := strings.IndexAny(raw, "eE"); at >= 0 {
			exponent, err := strconv.Atoi(raw[at+1:])
			if err != nil || exponent < -100 || exponent > 100 {
				invalid("Amount exponent exceeds limit")
				continue
			}
		}
		number, ok := new(big.Rat).SetString(raw)
		if !ok {
			invalid("Amount must be an exact decimal number")
			continue
		}
		unit, ok := aiJSONPointer(byID[c.UnitSourceID], c.UnitLocator)
		unitText, isString := unit.(string)
		if !ok || !isString || unitText != c.Unit {
			invalid("Unit does not match its cited source")
			continue
		}
		switch c.Unit {
		case "major":
		case "minor":
			if !number.IsInt() {
				invalid("Minor-unit amount must be an integer")
				continue
			}
			if c.Currency != "EUR" {
				invalid("Minor-unit conversion supports EUR only")
				continue
			}
			number.Quo(number, big.NewRat(100, 1))
		default:
			invalid("Amount unit must be major or minor")
			continue
		}
		currency, ok := aiJSONPointer(byID[c.CurrencySourceID], c.CurrencyLocator)
		currencyText, isString := currency.(string)
		if !ok || !isString || !aiCurrencyPattern.MatchString(c.Currency) || currencyText != c.Currency {
			invalid("Currency does not match its cited source")
			continue
		}
		// A decimal source divided by 100 has only factors 2 and 5 in its denominator.
		den := new(big.Int).Set(number.Denom())
		two := big.NewInt(2)
		five := big.NewInt(5)
		rem := new(big.Int)
		twos, fives := 0, 0
		for {
			q := new(big.Int)
			q.QuoRem(den, two, rem)
			if rem.Sign() != 0 {
				break
			}
			den = q
			twos++
		}
		for {
			q := new(big.Int)
			q.QuoRem(den, five, rem)
			if rem.Sign() != 0 {
				break
			}
			den = q
			fives++
		}
		if den.Cmp(big.NewInt(1)) != 0 {
			invalid("Amount cannot be represented exactly as decimal")
			continue
		}
		scale := max(twos, fives)
		if c.Unit == "minor" && scale < 2 {
			scale = 2
		}
		decimal := models.TracfinDecimal(number.FloatString(scale))
		value.Decimal = &decimal
		value.Explanation = strings.TrimSpace(value.Explanation + fmt.Sprintf(" Backend: %s %s units = %s %s, using the cited amount, unit and currency.", raw, c.Unit, decimal, c.Currency))
		value.Sources = append(value.Sources, AIEvidence{SourceID: c.SourceID, Locator: c.Locator}, AIEvidence{SourceID: c.UnitSourceID, Locator: c.UnitLocator}, AIEvidence{SourceID: c.CurrencySourceID, Locator: c.CurrencyLocator})
	}
	return diagnostics
}

func aiJSONPointer(raw json.RawMessage, pointer string) (any, bool) {
	if len(raw) == 0 || !strings.HasPrefix(pointer, "/") {
		return nil, false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var current any
	if decoder.Decode(&current) != nil {
		return nil, false
	}
	for _, part := range strings.Split(pointer[1:], "/") {
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		switch v := current.(type) {
		case map[string]any:
			var ok bool
			current, ok = v[part]
			if !ok {
				return nil, false
			}
		case []any:
			i, err := strconv.Atoi(part)
			if err != nil || i < 0 || i >= len(v) {
				return nil, false
			}
			current = v[i]
		default:
			return nil, false
		}
	}
	return current, true
}

// ValidateAIAmountTotals compares the accepted aggregate with all principal-operation
// totals using exact arithmetic, only when their shared currency is established.
func ValidateAIAmountTotals(d models.TracfinDraft, c Catalog) []models.TracfinDiagnostic {
	currency := func(field string) string { return TargetCurrency(d, c, field) }
	var diagnostics []models.TracfinDiagnostic
	WalkBlocks(d.Document.Root, func(b models.TracfinBlock) {
		if b.CatalogID != "/ds/operations" {
			return
		}
		totalField := "/ds/operations/montantTotal"
		operationField := "/ds/operations/operation/montantTotal"
		values := b.Fields[totalField]
		if len(values) != 1 || values[0].Decimal == nil {
			return
		}
		declared, ok := new(big.Rat).SetString(string(*values[0].Decimal))
		if !ok {
			return
		}
		sum := new(big.Rat)
		count := 0
		for _, op := range b.Children {
			if op.CatalogID != "/ds/operations/operation" {
				continue
			}
			amounts := op.Fields[operationField]
			if len(amounts) != 1 || amounts[0].Decimal == nil {
				return
			}
			amount, valid := new(big.Rat).SetString(string(*amounts[0].Decimal))
			if !valid {
				return
			}
			sum.Add(sum, amount)
			count++
		}
		if count == 0 {
			return
		}
		ref := &models.TracfinFieldRef{BlockID: b.ID, FieldID: totalField, ValueID: values[0].ID}
		if currency(totalField) == "" || currency(totalField) != currency(operationField) {
			diagnostics = append(diagnostics, aiDiagnostic("AMOUNT_TOTAL_CURRENCY_UNPROVEN", "Aggregate and principal-operation currencies must be explicitly established and equal", ref))
			return
		}
		if declared.Cmp(sum) != 0 {
			diagnostics = append(diagnostics, aiDiagnostic("AMOUNT_TOTAL_MISMATCH", "Aggregate amount differs from the exact backend sum of accepted principal operations", ref))
		}
	})
	return diagnostics
}
