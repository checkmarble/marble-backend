package tracfin

import (
	"sort"
	"strings"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

const (
	freeTextMaxLength = 200
)

var _ core.ItemRemovalRewriter = (*format)(nil)

// referenceIdentityFields are the fields of a financial reference that identify
// it as text (the IBAN or the number), in order of preference.
var referenceIdentityFields = []string{
	"iban", "numeroCompte", "numeroCarteBancaire", "NumeroCartePrepayee", "NumeroCheque", "NumeroContrat",
	"NumeroReference", "CryptoActif", "CagnotteEnLigne", "PortefeuilleElectronique", "bic",
}

// RewriteBeforeRemoval implements core.ItemRemovalRewriter for a person about to
// be removed (the analyst declines to declare it): everything that refers to it
// is rewritten so that nothing dangles once it is gone.
//
//   - in each working transaction, a reference to the person as emitter or
//     beneficiary becomes free text: emetteurLibre / beneficiaireLibre take the
//     person's name as filled (first name and name, or company name), and
//     refEmission / refReception, when they cite a financial reference of the
//     person, take the IBAN or number of that reference as text;
//   - any other value that points at the person or at something under it is
//     cleared (computed values are left: the operation lines are recomputed
//     from the rewritten transactions).
//
// The fills are computed from the document as it is now, never at question
// time: the transactions are filled after the placement.
func (f *format) RewriteBeforeRemoval(doc *core.Document, item core.Path) []core.Fill {
	if !strings.HasPrefix(string(item), "personnes.") {
		return nil
	}
	name := f.personName(doc, item)
	var fills []core.Fill
	handled := map[core.Path]bool{}
	put := func(p core.Path, v string) {
		handled[p] = true
		fills = append(fills, core.Fill{Path: p, Value: v})
	}
	pointsAtItem := func(v string) bool { return v != "" && core.Path(v).HasPrefix(item) }

	for _, k := range doc.Items[transactionsContainer] {
		tx := core.Path(string(transactionsContainer) + "[" + k + "]")
		for _, side := range []struct{ person, free, ref string }{
			{"emetteurPersonne", "emetteurLibre", "refEmission"},
			{"beneficiairePersonne", "beneficiaireLibre", "refReception"},
		} {
			if pointsAtItem(doc.Values[tx.Child(side.person)].Value) {
				put(tx.Child(side.person), "")
				put(tx.Child(side.free), truncate(name))
			}
			if ref := doc.Values[tx.Child(side.ref)].Value; pointsAtItem(ref) {
				put(tx.Child(side.ref), truncate(f.referenceText(doc, core.Path(ref))))
			}
		}
	}

	// Every other reference to the person is cleared (never inside the item, which
	// goes away with it, nor a computed field, recomputed after the removal).
	var others []core.Path
	for p, v := range doc.Values {
		if v.Value == "" || p.HasPrefix(item) || handled[p] || !pointsAtItem(v.Value) {
			continue
		}
		if node := f.spec.NodeAt(p.Template()); node != nil && node.EffectiveNature() == core.NatureComputed {
			continue
		}
		others = append(others, p)
	}
	sort.Slice(others, func(a, b int) bool { return others[a] < others[b] })
	for _, p := range others {
		put(p, "")
	}
	return fills
}

func truncate(s string) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) > freeTextMaxLength {
		r = r[:freeTextMaxLength]
	}
	return string(r)
}

// personName is the name of a declared person as filled: first name and name of
// a natural person, company name of a legal entity, identification elements of
// an uncharacterized person; the source objects when nothing is filled yet.
func (f *format) personName(doc *core.Document, person core.Path) string {
	val := func(p core.Path) string { return strings.TrimSpace(doc.Values[p].Value) }
	var name string
	switch {
	case strings.HasPrefix(string(person), "personnes.pp["):
		civil := person.Child("etatCivil")
		var first string
		for _, k := range doc.Items[civil.Child("prenom")] {
			if first = val(core.Path(string(civil.Child("prenom")) + "[" + k + "]")); first != "" {
				break
			}
		}
		last := val(civil.Child("nomNaissance"))
		if last == "" {
			last = val(civil.Child("nomDusage"))
		}
		name = strings.TrimSpace(first + " " + last)
	case strings.HasPrefix(string(person), "personnes.pm["):
		name = val(person.Child("activiteCoordonnee").Child("denominationSociale"))
	default:
		name = val(person.Child("elementsIdentification"))
	}
	if name == "" {
		var objs []string
		for _, o := range doc.ItemSources[person] {
			objs = append(objs, o.Table+"/"+o.ObjectID)
		}
		name = strings.Join(objs, ", ")
	}
	return name
}

// referenceText is the identifier of a declared financial reference as text: its
// IBAN or number. Empty when the reference holds none (cash, unknown reference).
func (f *format) referenceText(doc *core.Document, ref core.Path) string {
	for _, name := range referenceIdentityFields {
		if v := strings.TrimSpace(doc.Values[ref.Child(name)].Value); v != "" {
			return v
		}
	}
	return ""
}
