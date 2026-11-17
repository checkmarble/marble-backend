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
func (f *format) Compute(doc *core.Document) core.ComputeResult {
	return Compute(doc)
}

// Compute is the format-independent entry of f.Compute.
func Compute(doc *core.Document) core.ComputeResult {
	var res core.ComputeResult
	groups, order, minDate, maxDate, total, totalFrac, refFills := groupTransactions(doc)
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
	res.RemoveItems = stale
	return res
}

// groupTransactions builds the groups of the usable transactions, in order of
// first appearance, with the period, the total and the derived references.
func groupTransactions(doc *core.Document) (groups map[string]*opGroup, order []string, minDate, maxDate string,
	total *big.Rat, totalFrac int, refFills []core.Fill,
) {
	groups = map[string]*opGroup{}
	total = new(big.Rat)
	for _, k := range doc.Items[transactionsContainer] {
		tx := core.Path(string(transactionsContainer) + "[" + k + "]")
		val := func(name string) string { return strings.TrimSpace(doc.Values[tx.Child(name)].Value) }
		if d := val("devise"); d != "" && d != "EUR" {
			continue
		}
		amount, frac, ok := parseDecimal(val("montant"))
		if !ok {
			continue
		}
		emitter := counterparty(val("emetteurPersonne"), val("emetteurLibre"))
		beneficiary := counterparty(val("beneficiairePersonne"), val("beneficiaireLibre"))
		refFrom, refTo := val("refEmission"), val("refReception")
		if refFrom == "" {
			if ref := derivedFinancialRef(doc, tx, val("emetteurPersonne")); ref != "" {
				refFrom = string(ref)
				refFills = append(refFills, core.Fill{Path: tx.Child("refEmission"), Value: refFrom,
					Origin: core.OriginComputed, Computation: &core.Computation{Name: computationOperations}})
			}
		}
		if refTo == "" {
			if ref := derivedFinancialRef(doc, tx, val("beneficiairePersonne")); ref != "" {
				refTo = string(ref)
				refFills = append(refFills, core.Fill{Path: tx.Child("refReception"), Value: refTo,
					Origin: core.OriginComputed, Computation: &core.Computation{Name: computationOperations}})
			}
		}
		attrs := map[string]string{
			lineType: val("typeOperation"), lineOtherType: val("autretypeOperation"),
			lineEmitter: emitter, lineRefFrom: refFrom,
			lineBeneficiry: beneficiary, lineRefTo: refTo,
			lineCountryFrom: val("paysOrigine"), lineCountryTo: val("paysArrivee"),
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

func validDate(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

// strictDecimalRe is a plain decimal: digits, optionally a dot and digits (no
// fraction "1/2", no exponent, no sign, no separator).
var strictDecimalRe = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?$`)

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
