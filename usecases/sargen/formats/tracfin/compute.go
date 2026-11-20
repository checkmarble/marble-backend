package tracfin

import (
	"fmt"
	"math/big"
	"regexp"
	"slices"
	"sort"
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
	workCredits           core.Path = "work.montantCredits"
	workDebits            core.Path = "work.montantDebits"
	workChosenDirection   core.Path = "work.sensMontantTotal"

	directionDebit  = "debit"
	directionCredit = "credit"

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
// Direction and amount at stake (TRACFIN focus sheet « Le montant en jeu »):
// debits and credits are never added together. The direction of a transaction is
// its sens; when empty (or computed) it is derived: debit when the emitter is a
// declared person and the beneficiary is not, credit in the opposite case,
// unknown otherwise. The totals of the countable credits and of the countable
// debits go to work.montantCredits and work.montantDebits (computed, not
// exported). operations.montantTotal is the total of the direction chosen at the
// judgment step (work.sensMontantTotal), else the larger of the two; the
// transactions of unknown direction count in neither, and when none has a known
// direction the total is the sum of all of them (see stake).
//
// A line keeps its path while it stands for a group (see matchLines), also when
// the group attributes change but its transactions carry on; a new group gets a
// fresh key, never one given before in the document (core.Document.AddItem).
func (f *format) Compute(doc *core.Document) core.ComputeResult {
	var res core.ComputeResult
	groups, order, minDate, maxDate, st, refFills := groupTransactions(f.spec, doc)
	res.Fills = append(res.Fills, refFills...)

	if len(groups) == 0 {
		res.Fills = nil
		res.RemoveItems = removeComputedOperations(doc)
		return res
	}

	comp := func(inputs []core.Path) *core.Computation {
		return &core.Computation{Name: computationOperations, Inputs: inputs}
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
	total, totalFrac, totalTxs, basis := st.total(chosenDirection(doc))
	totalInputs := txAmounts(totalTxs)
	if basis == basisJudgment {
		totalInputs = append(totalInputs, workChosenDirection)
	}
	put(operationsTotal, formatDecimal(total, totalFrac), totalInputs)
	// The two direction totals: written while a transaction has that direction,
	// a stale computed one goes away.
	for _, d := range []struct {
		path core.Path
		side *directionTotal
	}{{workCredits, &st.credits}, {workDebits, &st.debits}} {
		if len(d.side.txs) > 0 {
			put(d.path, formatDecimal(d.side.sum, d.side.frac), txAmounts(d.side.txs))
		} else if v, ok := doc.Values[d.path]; ok && v.Origin == core.OriginComputed {
			res.RemoveItems = append(res.RemoveItems, d.path)
		}
	}

	// Match the existing lines with the groups: a line keeps its key (its path)
	// for as long as it stands for a group.
	matched, stale := matchLines(doc, groups, order)
	provisional := 0
	for _, k := range order {
		g := groups[k]
		line, ok := matched[k]
		if !ok {
			provisional++
			line = core.Path(fmt.Sprintf("%s[+l%d]", operationsContainer, provisional))
		}
		for _, name := range lineAttributes {
			if v := g.attrs[name]; v != "" {
				put(line.Child(name), v, txAmounts(g.txs))
			} else if cur, has := doc.Values[line.Child(name)]; has && ok && cur.Origin == core.OriginComputed {
				// The line follows its group to attributes that no longer have
				// this one: the stale computed value goes away.
				res.RemoveItems = append(res.RemoveItems, line.Child(name))
			}
		}
		put(line.Child(lineAmount), formatDecimal(g.sum, g.maxFrac), txAmounts(g.txs))
		put(line.Child(lineCount), fmt.Sprint(g.count), g.txs)
	}
	res.RemoveItems = append(res.RemoveItems, stale...)
	return res
}

// matchLines pairs the existing operation lines with the groups of the usable
// transactions, so that a line keeps its key as long as it stands for a group.
// A line is identified, in this order, by:
//  1. its stored line attributes: the line whose attributes are the group key;
//  2. the transactions it summarises (the inputs of its nombreOperations): a
//     group that took over transactions of a line whose attributes no longer
//     match (a person removed, a transaction edited, a regrouping) is that line.
//     The best overlap wins, one line per group, in line then group order.
//
// The stored attributes are only a hint: a value the analyst typed on a line
// attribute stays on its line, which is found again by its transactions. A line
// matched by nothing is stale; a group matched by nothing gets a new key (the
// document counter never gives a removed key again).
func matchLines(doc *core.Document, groups map[string]*opGroup, order []string) (matched map[string]core.Path, stale []core.Path) {
	matched = map[string]core.Path{}
	taken := map[core.Path]bool{}
	var lines []core.Path
	for _, key := range doc.Items[operationsContainer] {
		line := core.Path(string(operationsContainer) + "[" + key + "]")
		lines = append(lines, line)
		gk := lineKey(func(name string) string { return doc.Values[line.Child(name)].Value })
		if _, dup := matched[gk]; !dup && groups[gk] != nil {
			matched[gk] = line
			taken[line] = true
		}
	}
	type pair struct{ line, group, overlap int }
	var pairs []pair
	for li, line := range lines {
		if taken[line] {
			continue
		}
		var inputs []core.Path
		if c := doc.Values[line.Child(lineCount)].Computation; c != nil {
			inputs = c.Inputs
		}
		for gi, k := range order {
			if _, done := matched[k]; done {
				continue
			}
			n := 0
			for _, tx := range groups[k].txs {
				if slices.Contains(inputs, tx) {
					n++
				}
			}
			if n > 0 {
				pairs = append(pairs, pair{li, gi, n})
			}
		}
	}
	sort.SliceStable(pairs, func(a, b int) bool { return pairs[a].overlap > pairs[b].overlap })
	for _, p := range pairs {
		k := order[p.group]
		if _, done := matched[k]; done || taken[lines[p.line]] {
			continue
		}
		matched[k] = lines[p.line]
		taken[lines[p.line]] = true
	}
	for _, line := range lines {
		if !taken[line] {
			stale = append(stale, line)
		}
	}
	return matched, stale
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

// directionTotal is the sum of the countable transactions of one direction.
type directionTotal struct {
	sum  *big.Rat
	frac int
	txs  []core.Path
}

func (d *directionTotal) add(tx core.Path, amount *big.Rat, frac int) {
	if d.sum == nil {
		d.sum = new(big.Rat)
	}
	d.sum.Add(d.sum, amount)
	d.frac = max(d.frac, frac)
	d.txs = append(d.txs, tx)
}

func (d *directionTotal) value() *big.Rat {
	if d.sum == nil {
		return new(big.Rat)
	}
	return d.sum
}

// stake is what the countable transactions add up to, by direction.
type stake struct {
	credits, debits, all directionTotal
	// unknown is the number of countable transactions whose direction is not known.
	unknown int
}

func (s *stake) add(tx core.Path, amount *big.Rat, frac int, direction string) {
	s.all.add(tx, amount, frac)
	switch direction {
	case directionCredit:
		s.credits.add(tx, amount, frac)
	case directionDebit:
		s.debits.add(tx, amount, frac)
	default:
		s.unknown++
	}
}

// Why a direction was kept for the amount at stake.
const (
	basisJudgment = "judgment"
	basisLarger   = "larger"
	basisAll      = "all"
)

// total is the amount at stake: the total of the chosen direction when it has
// transactions, else the larger of the two (credits on a tie), else, when no
// transaction has a known direction, the sum of all of them. It returns the
// total, its fraction digits, the transactions it counts and why that
// direction was kept.
func (s *stake) total(chosen string) (total *big.Rat, frac int, txs []core.Path, basis string) {
	side := func(d *directionTotal) (*big.Rat, int, []core.Path) { return d.value(), d.frac, d.txs }
	switch {
	case len(s.credits.txs) == 0 && len(s.debits.txs) == 0:
		total, frac, txs = side(&s.all)
		return total, frac, txs, basisAll
	case chosen == directionCredit && len(s.credits.txs) > 0:
		total, frac, txs = side(&s.credits)
		return total, frac, txs, basisJudgment
	case chosen == directionDebit && len(s.debits.txs) > 0:
		total, frac, txs = side(&s.debits)
		return total, frac, txs, basisJudgment
	case s.credits.value().Cmp(s.debits.value()) >= 0:
		total, frac, txs = side(&s.credits)
	default:
		total, frac, txs = side(&s.debits)
	}
	return total, frac, txs, basisLarger
}

// chosenDirection is the direction the judgment step (or the analyst) chose for
// the amount at stake; empty when none or not a direction.
func chosenDirection(doc *core.Document) string {
	switch d := strings.TrimSpace(doc.Values[workChosenDirection].Value); d {
	case directionCredit, directionDebit:
		return d
	}
	return ""
}

// transactionDirection derives the direction of a transaction from who is a
// declared person: debit when only the emitter is, credit when only the
// beneficiary is, unknown otherwise.
func transactionDirection(emitterPerson, beneficiaryPerson string) string {
	switch {
	case emitterPerson != "" && beneficiaryPerson == "":
		return directionDebit
	case beneficiaryPerson != "" && emitterPerson == "":
		return directionCredit
	}
	return ""
}

// groupTransactions builds the groups of the usable transactions, in order of
// first appearance, with the period, the totals by direction and the derived
// references and directions.
func groupTransactions(spec *core.Spec, doc *core.Document) (groups map[string]*opGroup, order []string, minDate, maxDate string,
	st stake, refFills []core.Fill,
) {
	groups = map[string]*opGroup{}
	unknownCountry := ""
	if node := spec.NodeAt("work.transactions[].paysOrigine"); node != nil {
		unknownCountry = spec.CodeLists[node.CodeList].UnknownCode
	}
	for _, k := range doc.Items[transactionsContainer] {
		tx := core.Path(string(transactionsContainer) + "[" + k + "]")
		val := func(name string) string { return strings.TrimSpace(doc.Values[tx.Child(name)].Value) }
		// derived returns the value a derivable field has once derivation ran, and
		// the fill to write: derivation applies when the field is empty (and not
		// cleared by hand) or holds a computed value.
		derived := func(name, derivation string, inputs ...core.Path) string {
			cur := doc.Values[tx.Child(name)]
			// The unknown country the model wrote when the transaction row says
			// nothing yields to a derivation from the IBAN, as long as nobody
			// reviewed or typed it.
			if derivation != "" && unknownCountry != "" && cur.Value == unknownCountry &&
				(name == "paysOrigine" || name == "paysArrivee") && cur.Origin == core.OriginLLM && !cur.Reviewed {
				cur = core.Value{}
			}
			if cur.Value != "" && cur.Origin != core.OriginComputed {
				return strings.TrimSpace(cur.Value)
			}
			if cur.Value == "" && cur.Origin != "" { // manual or analyst clear
				return ""
			}
			if derivation != cur.Value {
				refFills = append(refFills, core.Fill{Path: tx.Child(name), Value: derivation,
					Origin: core.OriginComputed, Computation: &core.Computation{Name: computationOperations, Inputs: inputs}})
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
		direction := derived("sens", transactionDirection(val("emetteurPersonne"), val("beneficiairePersonne")),
			tx.Child("emetteurPersonne"), tx.Child("beneficiairePersonne"))
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
		st.add(tx, amount, frac, direction)
		if d := val("date"); validDate(d) {
			if minDate == "" || d < minDate {
				minDate = d
			}
			if d > maxDate {
				maxDate = d
			}
		}
	}
	return groups, order, minDate, maxDate, st, refFills
}

// removeComputedOperations lists the computed operations values and lines to
// remove when no transaction is usable. Values the analyst wrote stay.
func removeComputedOperations(doc *core.Document) []core.Path {
	var out []core.Path
	for _, p := range []core.Path{operationsPeriodStart, operationsPeriodEnd, operationsTotal, workCredits, workDebits} {
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

// ComputedNote is the message that goes with a computed value the analyst is
// asked to confirm. For the total of the operations it gives the total of the
// credits, the total of the debits, the direction kept and why, and the number
// of transactions without a direction (they count in neither total).
func (f *format) ComputedNote(doc *core.Document, p core.Path) string {
	if p != operationsTotal || doc == nil {
		return ""
	}
	var st stake
	for _, k := range doc.Items[transactionsContainer] {
		tx := core.Path(string(transactionsContainer) + "[" + k + "]")
		val := func(name string) string { return strings.TrimSpace(doc.Values[tx.Child(name)].Value) }
		amount, frac, status := countableTransaction(val)
		if status != txCountable {
			continue
		}
		st.add(tx, amount, frac, val("sens"))
	}
	chosen := chosenDirection(doc)
	_, _, _, basis := st.total(chosen)
	money := func(d *directionTotal) string { return formatDecimal(d.value(), d.frac) + " EUR" }
	label := map[string]string{directionCredit: "crédit", directionDebit: "débit"}
	msg := fmt.Sprintf("valeur calculée à confirmer : le montant total. Crédits : %s ; débits : %s", money(&st.credits), money(&st.debits))
	switch basis {
	case basisAll:
		msg += ". Aucune transaction n'a de sens connu : le total additionne toutes les transactions comptées, crédits et débits confondus ; " +
			"renseignez le sens des transactions pour ne retenir qu'un sens"
	case basisJudgment:
		msg += ". Sens retenu : " + label[chosen] + " (choisi à l'étape de jugement)"
	default:
		reason := "le plus élevé, faute de choix"
		if chosen != "" {
			reason = "le plus élevé : aucune transaction dans le sens choisi à l'étape de jugement"
		}
		kept := directionCredit
		if st.credits.value().Cmp(st.debits.value()) < 0 {
			kept = directionDebit
		}
		msg += ". Sens retenu : " + label[kept] + " (" + reason + ")"
	}
	return msg + fmt.Sprintf(". Transactions sans sens : %d (comptées dans aucun total).", st.unknown)
}
