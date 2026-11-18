package tracfin

import (
	"fmt"
	"math/big"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

const (
	transactionsContainer core.Path = "work.transactions"
	operationsContainer   core.Path = "operations.operation"
	operationsPeriodStart core.Path = "operations.dateDebut"
	operationsPeriodEnd   core.Path = "operations.dateFin"
	operationsTotal       core.Path = "operations.montantTotal"

	computationOperations = "operations_from_transactions"
	maxOperationLines     = 30
	msgTooManyOperations  = "Plus de %d lignes d'opération (%d présentes) : regroupez des opérations : modifiez les attributs de ligne des transactions."
)

// lineFields maps the operation-line attributes (the group key) to the
// transaction fields they are copied from. Counterparties are special-cased.
const (
	lineType, lineOtherType     = "typeOperation", "autretypeOperation"
	lineEmitter, lineBeneficiry = "emetteurTypeEmetteur", "beneficiaireTypeBeneficiaire"
	lineRefFrom, lineRefTo      = "refFinanciereEmission", "refFinanciereReception"
	lineCountryFrom             = "paysOrigineFonds"
	lineCountryTo               = "paysArriveeFonds"
	lineAmount, lineCount       = "montantTotal", "nombreOperations"
)

// lineAttributes are the group key fields of an operation line, in key order.
var lineAttributes = []string{
	lineType, lineOtherType, lineEmitter, lineRefFrom, lineBeneficiry, lineRefTo, lineCountryFrom, lineCountryTo,
}

// opGroup is one future operation line: the transactions sharing the same line
// attributes.
type opGroup struct {
	attrs   map[string]string
	txs     []core.Path
	sum     *big.Rat
	count   int
	maxFrac int
}

// Compute derives the operations block from work.transactions: the lines
// (grouped by their line attributes), the period and the total.
//
// Group key: typeOperation, emetteurPersonne or emetteurLibre, refEmission,
// beneficiairePersonne or beneficiaireLibre, refReception, paysOrigine,
// paysArrivee, plus autretypeOperation (two "other" operations described
// differently are not the same line). A counterparty that is an item path stays
// a reference on the line, free text stays text. Transactions in a currency other
// than EUR are excluded (the validator blocks them), as are transactions
// without a usable amount. An existing line whose attributes are those of a
// group is reused, so its path never changes; a line without a group is listed
// in RemoveItems. Without any usable transaction nothing is written and the
// computed period, total and lines are removed (the operations block is
// optional in the XSD).
//
// refEmission / refReception left empty on a transaction are derived when a
// source object of the transaction is itself a source object of a declared
// financial reference of its emitter / beneficiary person (the data model is
// not available here: no link is followed; otherwise the LLM or the analyst
// fills them).
//
// paysOrigine / paysArrivee left empty are derived from the country code (first
// two letters) of the IBAN of the emitter's / beneficiary's financial reference,
// when it is in the country list. A derived value is written as computed: it is
// recomputed while it stays computed, and never replaces a value written by the
// analyst or the model, nor a field the analyst cleared by hand.
//
// A line whose group attributes change gets a NEW path (the old line is removed):
// questions that cite the old line path become stale.
func (f *format) Compute(doc *core.Document) core.ComputeResult {
	var res core.ComputeResult
	groups, order, minDate, maxDate, total, totalFrac, refFills := groupTransactions(f.spec, doc)
	res.Fills = append(res.Fills, refFills...)

	if len(groups) == 0 {
		res.Fills = nil
		res.RemoveItems = removeComputedOperations(doc)
		return res
	}

	comp := func(inputs []core.Path) *core.Computation {
		return &core.Computation{Name: computationOperations, Inputs: inputs}
	}
	var allInputs []core.Path
	for _, k := range order {
		allInputs = append(allInputs, groups[k].txs...)
	}
	put := func(p core.Path, v string, inputs []core.Path) {
		res.Fills = append(res.Fills, core.Fill{Path: p, Value: v, Origin: core.OriginComputed, Computation: comp(inputs)})
	}
	txAmounts := func(txs []core.Path) []core.Path {
		out := make([]core.Path, len(txs))
		for i, t := range txs {
			out[i] = t.Child("montant")
		}
		return out
	}
	if minDate != "" {
		put(operationsPeriodStart, minDate, nil)
		put(operationsPeriodEnd, maxDate, nil)
	} else {
		// No valid date remains: a stale computed period goes away.
		for _, p := range []core.Path{operationsPeriodStart, operationsPeriodEnd} {
			if v, ok := doc.Values[p]; ok && v.Origin == core.OriginComputed {
				res.RemoveItems = append(res.RemoveItems, p)
			}
		}
	}
	put(operationsTotal, formatDecimal(total, totalFrac), txAmounts(allInputs))

	// Match the existing lines with the groups.
	existing := map[string]core.Path{}
	var stale []core.Path
	for _, key := range doc.Items[operationsContainer] {
		line := core.Path(string(operationsContainer) + "[" + key + "]")
		gk := lineKey(func(name string) string { return doc.Values[line.Child(name)].Value })
		if _, taken := existing[gk]; taken || groups[gk] == nil {
			stale = append(stale, line)
			continue
		}
		existing[gk] = line
	}
	provisional := 0
	for _, k := range order {
		g := groups[k]
		line, ok := existing[k]
		if !ok {
			provisional++
			line = core.Path(fmt.Sprintf("%s[+l%d]", operationsContainer, provisional))
		}
		for _, name := range lineAttributes {
			if v := g.attrs[name]; v != "" {
				put(line.Child(name), v, txAmounts(g.txs))
			}
		}
		put(line.Child(lineAmount), formatDecimal(g.sum, g.maxFrac), txAmounts(g.txs))
		put(line.Child(lineCount), fmt.Sprint(g.count), g.txs)
	}
	res.RemoveItems = append(res.RemoveItems, stale...)
	return res
}

// txStatus tells whether Compute counts a working transaction, and if not why.
type txStatus int

const (
	txCountable txStatus = iota
	// txForeignCurrency: a currency other than EUR (excluded on purpose; the
	// montant_hors_euro rule blocks it).
	txForeignCurrency
	// txNoCurrency: no currency, which is not EUR either.
	txNoCurrency
	// txNoAmount: no amount, or one that is not a decimal.
	txNoAmount
)

// countableTransaction is the single rule shared by Compute and Validate: a
// transaction is counted when its currency is EUR and its amount is a decimal.
// val returns the trimmed value of a transaction field.
func countableTransaction(val func(name string) string) (amount *big.Rat, frac int, status txStatus) {
	switch d := val("devise"); d {
	case "EUR":
	case "":
		return nil, 0, txNoCurrency
	default:
		return nil, 0, txForeignCurrency
	}
	amount, frac, ok := parseDecimal(val("montant"))
	if !ok {
		return nil, 0, txNoAmount
	}
	return amount, frac, txCountable
}

// groupTransactions builds the groups of the usable transactions, in order of
// first appearance, with the period, the total and the derived references.
func groupTransactions(spec *core.Spec, doc *core.Document) (groups map[string]*opGroup, order []string, minDate, maxDate string,
	total *big.Rat, totalFrac int, refFills []core.Fill,
) {
	groups = map[string]*opGroup{}
	total = new(big.Rat)
	for _, k := range doc.Items[transactionsContainer] {
		tx := core.Path(string(transactionsContainer) + "[" + k + "]")
		val := func(name string) string { return strings.TrimSpace(doc.Values[tx.Child(name)].Value) }
		// derived returns the value a derivable field has once derivation ran, and
		// the fill to write: derivation applies when the field is empty (and not
		// cleared by hand) or holds a computed value.
		derived := func(name, derivation string) string {
			cur := doc.Values[tx.Child(name)]
			if cur.Value != "" && cur.Origin != core.OriginComputed {
				return strings.TrimSpace(cur.Value)
			}
			if cur.Value == "" && cur.Origin != "" { // manual or analyst clear
				return ""
			}
			if derivation != cur.Value {
				refFills = append(refFills, core.Fill{Path: tx.Child(name), Value: derivation,
					Origin: core.OriginComputed, Computation: &core.Computation{Name: computationOperations}})
			}
			return derivation
		}
		amount, frac, status := countableTransaction(val)
		if status != txCountable {
			continue
		}
		emitter := counterparty(val("emetteurPersonne"), val("emetteurLibre"))
		beneficiary := counterparty(val("beneficiairePersonne"), val("beneficiaireLibre"))
		refFrom := derived("refEmission", string(derivedFinancialRef(doc, tx, val("emetteurPersonne"))))
		refTo := derived("refReception", string(derivedFinancialRef(doc, tx, val("beneficiairePersonne"))))
		countryFrom := derived("paysOrigine", ibanCountry(spec, doc, refFrom))
		countryTo := derived("paysArrivee", ibanCountry(spec, doc, refTo))
		attrs := map[string]string{
			lineType: val("typeOperation"), lineOtherType: val("autretypeOperation"),
			lineEmitter: emitter, lineRefFrom: refFrom,
			lineBeneficiry: beneficiary, lineRefTo: refTo,
			lineCountryFrom: countryFrom, lineCountryTo: countryTo,
		}
		gk := lineKey(func(name string) string { return attrs[name] })
		g := groups[gk]
		if g == nil {
			g = &opGroup{attrs: attrs, sum: new(big.Rat)}
			groups[gk] = g
			order = append(order, gk)
		}
		g.txs = append(g.txs, tx)
		g.sum.Add(g.sum, amount)
		g.count++
		g.maxFrac = max(g.maxFrac, frac)
		total.Add(total, amount)
		totalFrac = max(totalFrac, frac)
		if d := val("date"); validDate(d) {
			if minDate == "" || d < minDate {
				minDate = d
			}
			if d > maxDate {
				maxDate = d
			}
		}
	}
	return groups, order, minDate, maxDate, total, totalFrac, refFills
}

// removeComputedOperations lists the computed operations values and lines to
// remove when no transaction is usable. Values the analyst wrote stay.
func removeComputedOperations(doc *core.Document) []core.Path {
	var out []core.Path
	for _, p := range []core.Path{operationsPeriodStart, operationsPeriodEnd, operationsTotal} {
		if v, ok := doc.Values[p]; ok && v.Origin == core.OriginComputed {
			out = append(out, p)
		}
	}
	for _, key := range doc.Items[operationsContainer] {
		out = append(out, core.Path(string(operationsContainer)+"["+key+"]"))
	}
	return out
}

// lineKey joins the line attributes into a comparable key.
func lineKey(get func(name string) string) string {
	parts := make([]string, len(lineAttributes))
	for i, name := range lineAttributes {
		parts[i] = strings.TrimSpace(get(name))
	}
	return strings.Join(parts, "\x00")
}

// counterparty is the item path of the referenced person when there is one, else
// the free text.
func counterparty(person, free string) string {
	if person != "" {
		return person
	}
	return free
}

// derivedFinancialRef returns the declared financial reference of person that
// shares a source object with the transaction, when exactly one does.
func derivedFinancialRef(doc *core.Document, tx core.Path, person string) core.Path {
	if person == "" {
		return ""
	}
	sources := doc.ItemSources[tx]
	if len(sources) == 0 {
		return ""
	}
	var found []core.Path
	for container, keys := range doc.Items {
		if !strings.HasPrefix(string(container), person+".referencesFinancieres.referenceFinanciere") {
			continue
		}
		for _, k := range keys {
			ref := core.Path(string(container) + "[" + k + "]")
			if slices.ContainsFunc(doc.ItemSources[ref], func(o core.ObjectRef) bool { return slices.Contains(sources, o) }) {
				found = append(found, ref)
			}
		}
	}
	if len(found) != 1 {
		return ""
	}
	return found[0]
}

var ibanPrefixRe = regexp.MustCompile(`^[A-Za-z]{2}[0-9]{2}`)

// ibanCountry returns the country code of the IBAN behind a financial reference
// (the path of a declared reference holding an `iban`, or the IBAN as text) when
// the code is in the country list, else "".
func ibanCountry(spec *core.Spec, doc *core.Document, ref string) string {
	iban := ref
	if v, ok := doc.Values[core.Path(ref).Child("iban")]; ok && strings.Contains(ref, "[") {
		iban = v.Value
	}
	iban = strings.ReplaceAll(strings.TrimSpace(iban), " ", "")
	if !ibanPrefixRe.MatchString(iban) || !validIBAN(iban) {
		return ""
	}
	code := strings.ToUpper(iban[:2])
	node := spec.NodeAt("work.transactions[].paysOrigine")
	if node == nil {
		return ""
	}
	for _, c := range spec.CodeLists[node.CodeList].Codes {
		if c.Value == code {
			return code
		}
	}
	return ""
}

// validIBAN checks the plausible length (15 to 34) and the mod-97 checksum.
func validIBAN(iban string) bool {
	if len(iban) < 15 || len(iban) > 34 {
		return false
	}
	rearranged := strings.ToUpper(iban[4:] + iban[:4])
	rem := 0
	for _, r := range rearranged {
		switch {
		case r >= '0' && r <= '9':
			rem = (rem*10 + int(r-'0')) % 97
		case r >= 'A' && r <= 'Z':
			rem = (rem*100 + int(r-'A'+10)) % 97
		default:
			return false
		}
	}
	return rem == 1
}

func validDate(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

// strictDecimalRe is a plain decimal: digits, optionally a dot and digits (no
// fraction "1/2", no exponent, no sign, no separator).
var strictDecimalRe = regexp.MustCompile(`^([0-9]+(\.[0-9]+)?|\.[0-9]+)$`)

// parseDecimal reads a positive decimal; frac is its number of fraction digits.
func parseDecimal(s string) (r *big.Rat, frac int, ok bool) {
	if !strictDecimalRe.MatchString(s) {
		return nil, 0, false
	}
	r, ok = new(big.Rat).SetString(s)
	if !ok || r.Sign() <= 0 {
		return nil, 0, false
	}
	if i := strings.IndexByte(s, '.'); i >= 0 {
		frac = len(s) - i - 1
	}
	return r, frac, true
}

// formatDecimal writes r with at most frac fraction digits, without trailing zeros.
func formatDecimal(r *big.Rat, frac int) string {
	s := r.FloatString(max(frac, 2))
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	return s
}

// tooManyOperationsIssue rewrites the generic maximum-items issue of the
// operation lines with the TRACFIN guidance.
func tooManyOperationsIssue(issues []core.Issue, doc *core.Document) {
	for i := range issues {
		if issues[i].Code == core.CodeTooManyItems && issues[i].Path == operationsContainer {
			issues[i].Message = fmt.Sprintf(msgTooManyOperations, maxOperationLines, len(doc.Items[operationsContainer]))
		}
	}
}

// ConfirmComputed lists the computed values the analyst confirms: the total of
// the operations is "à l'appréciation du déclarant" (core.ConfirmComputer).
func (f *format) ConfirmComputed() []core.Path {
	return []core.Path{operationsTotal}
}
