package tracfin

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

var (
	_ core.DocumentQuestioner = (*format)(nil)
	_ core.CodeSourceMapper   = (*format)(nil)
)

// docStake adds up the countable working transactions of the document by
// direction (their sens as stored: Compute has written the derived ones).
func docStake(doc *core.Document) stake {
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
	return st
}

// DocumentQuestions implements core.DocumentQuestioner: what the backend asks
// the analyst from the document alone, never writing anything.
//   - the direction of the amount at stake when both directions have
//     transactions and none was chosen, and the amount itself when no
//     transaction has a known direction (a blocking question);
//   - the counterparties written as free text whose IBAN is the account of a
//     declared person (a warning: the transaction exports with free text anyway).
func (f *format) DocumentQuestions(doc *core.Document) []core.QuestionDraft {
	if doc == nil {
		return nil
	}
	var out []core.QuestionDraft
	out = append(out, amountQuestions(doc)...)
	out = append(out, f.reconciliationQuestions(doc)...)
	return out
}

func amountQuestions(doc *core.Document) []core.QuestionDraft {
	st := docStake(doc)
	if len(st.all.txs) == 0 {
		return nil
	}
	if _, _, _, basis := st.total(chosenDirection(doc)); basis != basisAsk {
		return nil
	}
	if cur, ok := doc.Values[operationsTotal]; ok && cur.Value != "" {
		return nil // typed by the analyst: nothing to ask
	}
	if len(st.credits.txs) == 0 && len(st.debits.txs) == 0 {
		return []core.QuestionDraft{{
			Paths: []core.Path{operationsTotal}, Blocking: true,
			Text: fmt.Sprintf("Le sens (crédit ou débit) d'aucune des %d transactions n'est connu : le montant total en jeu ne peut pas être calculé. Indiquez-le.", len(st.all.txs)),
		}}
	}
	option := func(label string, d *directionTotal, direction string) core.QuestionOption {
		return core.QuestionOption{
			Label: fmt.Sprintf("%s : %s € (%d %s)", label, formatDecimal(d.value(), d.frac), len(d.txs), plural(len(d.txs), "transaction", "transactions")),
			Fills: []core.Fill{{Path: workChosenDirection, Value: direction, Origin: core.OriginAnalyst}},
		}
	}
	return []core.QuestionDraft{{
		Paths: []core.Path{workChosenDirection}, Blocking: true,
		Text: "Les transactions comptent des crédits et des débits, qui ne s'additionnent pas : quel sens retenir pour le montant total en jeu ?",
		Options: []core.QuestionOption{
			option("Crédits", &st.credits, directionCredit),
			option("Débits", &st.debits, directionDebit),
		},
	}}
}

func plural(n int, one, many string) string {
	if n > 1 {
		return many
	}
	return one
}

// ---- counterparty reconciliation ----

// declaredReference is a financial reference of a declared person that holds an
// IBAN.
type declaredReference struct {
	person, ref core.Path
	iban        string
}

func normIBAN(s string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(s), " ", ""))
}

// declaredIBANs lists the declared references that hold an IBAN, by normalized
// IBAN.
func declaredIBANs(doc *core.Document) map[string][]declaredReference {
	out := map[string][]declaredReference{}
	for container, keys := range doc.Items {
		m := refFinContainerRe.FindStringSubmatch(string(container))
		if m == nil {
			continue
		}
		for _, k := range keys {
			ref := core.Path(string(container) + "[" + k + "]")
			if iban := normIBAN(doc.Values[ref.Child("iban")].Value); iban != "" {
				out[iban] = append(out[iban], declaredReference{person: core.Path(m[1]), ref: ref, iban: iban})
			}
		}
	}
	return out
}

// reconcileSide is one side (issuer or beneficiary) of the transactions.
type reconcileSide struct {
	name, person, free, ref string
	article                 string
}

var reconcileSides = []reconcileSide{
	{name: "issuer", person: "emetteurPersonne", free: "emetteurLibre", ref: "refEmission", article: "l'émetteur"},
	{name: "beneficiary", person: "beneficiairePersonne", free: "beneficiaireLibre", ref: "refReception", article: "le bénéficiaire"},
}

type reconcileGroup struct {
	side  reconcileSide
	match declaredReference
	txs   []core.Path
}

// reconciliationQuestions asks, once per group (side, person, IBAN), whether a
// counterparty written as free text is the declared person whose account it
// reads. A match needs a full IBAN with a valid checksum, equal after
// normalization to the IBAN of a declared reference, of exactly one declared
// person. A partial or masked IBAN, an invalid one, an account number that is
// not an IBAN, or two persons holding it: no question.
func (f *format) reconciliationQuestions(doc *core.Document) []core.QuestionDraft {
	declared := declaredIBANs(doc)
	if len(declared) == 0 {
		return nil
	}
	groups := map[string]*reconcileGroup{}
	var order []string
	for _, k := range doc.Items[transactionsContainer] {
		tx := core.Path(string(transactionsContainer) + "[" + k + "]")
		val := func(name string) string { return strings.TrimSpace(doc.Values[tx.Child(name)].Value) }
		for _, side := range reconcileSides {
			if val(side.person) != "" || val(side.free) == "" {
				continue // linked to a declared person already, or no counterparty
			}
			iban := normIBAN(val(side.ref))
			if !ibanPrefixRe.MatchString(iban) || !validIBAN(iban) {
				continue
			}
			refs := declared[iban]
			persons := map[core.Path]bool{}
			for _, r := range refs {
				persons[r.person] = true
			}
			if len(persons) != 1 || len(refs) != 1 {
				continue
			}
			key := side.name + "\x00" + string(refs[0].person) + "\x00" + iban
			g := groups[key]
			if g == nil {
				g = &reconcileGroup{side: side, match: refs[0]}
				groups[key] = g
				order = append(order, key)
			}
			g.txs = append(g.txs, tx)
		}
	}
	sort.Strings(order)
	var out []core.QuestionDraft
	for _, key := range order {
		g := groups[key]
		name := f.personName(doc, g.match.person)
		var paths []core.Path
		var fills []core.Fill
		var clear []core.Path
		var objs []string
		for _, tx := range g.txs {
			paths = append(paths, tx.Child(g.side.person), tx.Child(g.side.free), tx.Child(g.side.ref))
			fills = append(fills,
				core.Fill{Path: tx.Child(g.side.person), Value: string(g.match.person), Origin: core.OriginAnalyst},
				core.Fill{Path: tx.Child(g.side.ref), Value: string(g.match.ref), Origin: core.OriginAnalyst},
			)
			clear = append(clear, tx.Child(g.side.free))
			objs = append(objs, sourceLabel(doc, tx))
		}
		var text string
		if g.side.name == "issuer" {
			text = fmt.Sprintf("Les virements %s viennent de l'IBAN %s, qui est le compte de %s, déclaré(e) dans cette déclaration. Rattacher %s à %s ?",
				listObjects(objs), g.match.iban, name, g.side.article, name)
		} else {
			text = fmt.Sprintf("Les virements %s vont vers l'IBAN %s, qui est le compte de %s, déclaré(e) dans cette déclaration. Rattacher %s à %s ?",
				listObjects(objs), g.match.iban, name, g.side.article, name)
		}
		out = append(out, core.QuestionDraft{
			Paths: paths, Text: text,
			Options: []core.QuestionOption{
				{Label: "Rattacher à " + name, Fills: fills, Clear: clear},
				{Label: "Laisser en texte libre"},
			},
		})
	}
	return out
}

// sourceLabel names a transaction by its source objects (table/id), else by its
// item key.
func sourceLabel(doc *core.Document, tx core.Path) string {
	var parts []string
	for _, o := range doc.ItemSources[tx] {
		parts = append(parts, o.Table+"/"+o.ObjectID)
	}
	if len(parts) == 0 {
		return string(tx)
	}
	return strings.Join(parts, "+")
}

// listObjects lists at most five names, then counts the rest.
func listObjects(objs []string) string {
	if len(objs) <= 5 {
		return strings.Join(objs, ", ")
	}
	return fmt.Sprintf("%s et %d autres", strings.Join(objs[:5], ", "), len(objs)-5)
}

// ---- code sources (core.CodeSourceMapper) ----

// CodeListSemantics implements core.CodeSourceMapper: the country lists are fed
// by the columns the client typed as a country (the value is matched to a code)
// or as an IBAN (the two-letter prefix of a valid IBAN).
func (f *format) CodeListSemantics(codeList string) []models.FieldSemanticType {
	if strings.Contains(codeList, "liste_ref_pays") {
		return []models.FieldSemanticType{models.FieldSemanticTypeCountry, models.FieldSemanticTypeIban}
	}
	return nil
}

// CodeFromValue implements core.CodeSourceMapper: an exact code of the list, or
// its label, ignoring case and accents; for an IBAN, its country prefix when the
// IBAN is valid.
func (f *format) CodeFromValue(codeList string, semantic models.FieldSemanticType, raw string) (string, bool) {
	list, ok := f.spec.CodeLists[codeList]
	if !ok {
		return "", false
	}
	if semantic == models.FieldSemanticTypeIban {
		iban := normIBAN(raw)
		if !ibanPrefixRe.MatchString(iban) || !validIBAN(iban) {
			return "", false
		}
		raw = iban[:2]
	}
	folded := foldCode(raw)
	if folded == "" {
		return "", false
	}
	for _, c := range list.Codes {
		if foldCode(c.Value) == folded && c.Value != list.UnknownCode {
			return c.Value, true
		}
	}
	for _, c := range list.Codes {
		if c.Label != "" && foldCode(c.Label) == folded && c.Value != list.UnknownCode {
			return c.Value, true
		}
	}
	return "", false
}

// foldCode lowercases, strips accents and trims.
func foldCode(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, strings.TrimSpace(s))
	if err != nil {
		out = s
	}
	return strings.ToLower(strings.TrimSpace(out))
}
