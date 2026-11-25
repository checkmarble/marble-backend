package tracfin

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

const (
	ruleCadreLegalPrincipal    = "cadre_legal_principal"
	rulePersonnesAuMoinsUne    = "personnes_au_moins_une"
	ruleReferencesFinMax20     = "references_financieres_max_20"
	ruleSousCategorieAutre     = "sous_categorie_relation_autre"
	ruleDirigeantBenefDistinct = "dirigeant_beneficiaire_distinct"
	ruleLienAutrePersonne      = "lien_autre_personne_distincte"
	rulePeriodeOperations      = "periode_operations_ordonnee"
	codeMontantHorsEuro        = "montant_hors_euro"
	maxReferencesFinancieres   = 20
	relationsCodeList          = "liste_ref_relations.csv"
	sousCategorieAutreSuffix   = ".typeRelation[].sousCategorieRelationAutre"
	sousCategorieSiblingField  = "sousCategorieRelation"
)

// HandRuleIDs lists the hand-written rules implemented in this file. A test
// checks it against `hand_rules` in constraints.json (nothing missing, nothing
// extra).
func HandRuleIDs() []string {
	return []string{ruleCadreLegalPrincipal, rulePersonnesAuMoinsUne, ruleReferencesFinMax20, ruleSousCategorieAutre, ruleDirigeantBenefDistinct,
		ruleLienAutrePersonne, rulePeriodeOperations}
}

var (
	cadreLegalPrincipalCodes = []string{"CL01", "CL02", "CL03"}
	refFinContainerRe        = regexp.MustCompile(`^(personnes\.(?:pp|pm|pnc)\[[^\]]+\])\.referencesFinancieres\.referenceFinanciere[A-Za-z]+$`)
)

// Validate runs the generic validator, then the TRACFIN rules that the
// generated spec cannot express (hand_rules in constraints.json) and the
// single-currency rule of the working transactions.
func Validate(spec *core.Spec, doc *core.Document) []core.Issue {
	issues := core.Validate(spec, doc)
	issues = append(issues, cadreLegalPrincipal(doc)...)
	issues = append(issues, personnesAuMoinsUne(doc)...)
	issues = append(issues, referencesFinancieresMax(doc)...)
	issues = append(issues, sousCategorieAutre(spec, doc)...)
	issues = append(issues, dirigeantBeneficiaireDistinct(doc)...)
	issues = append(issues, lienAutrePersonneDistincte(doc)...)
	issues = append(issues, periodeOperationsOrdonnee(doc)...)
	issues = append(issues, montantHorsEuro(doc)...)
	core.SortIssues(issues)
	return issues
}

func ruleIssue(id string, path core.Path, sev core.Severity, msg string, params ...string) core.Issue {
	return core.Issue{Path: path, Code: "rule:" + id, Message: msg, Severity: sev, RuleID: id, Params: core.Params(params...)}
}

func cadreLegalPrincipal(doc *core.Document) []core.Issue {
	const container core.Path = "context.natureSoupcon.cadreLegal"
	keys := doc.ItemKeys(container)
	if len(keys) == 0 {
		return nil
	}
	for _, k := range keys {
		v := doc.Values[core.Path(string(container)+"["+k+"]")].Value
		for _, c := range cadreLegalPrincipalCodes {
			if v == c {
				return nil
			}
		}
	}
	return []core.Issue{ruleIssue(ruleCadreLegalPrincipal, container, core.SeverityBlocking,
		"The legal framework must contain at least one of the codes CL01, CL02 or CL03.",
		"codes", strings.Join(cadreLegalPrincipalCodes, ","))}
}

func personnesAuMoinsUne(doc *core.Document) []core.Issue {
	present := false
	for p, v := range doc.Values {
		if v.Value != "" && p.HasPrefix("personnes") {
			present = true
		}
	}
	for _, p := range doc.ItemContainers() {
		if p.HasPrefix("personnes") {
			present = true
		}
	}
	if !present {
		return nil
	}
	if len(doc.ItemKeys("personnes.pp"))+len(doc.ItemKeys("personnes.pm"))+len(doc.ItemKeys("personnes.pnc")) > 0 {
		return nil
	}
	return []core.Issue{ruleIssue(rulePersonnesAuMoinsUne, "personnes", core.SeverityBlocking,
		"The persons section must contain at least one person (natural, legal or uncharacterised).")}
}

func referencesFinancieresMax(doc *core.Document) []core.Issue {
	totals := map[string]int{}
	for _, container := range doc.ItemContainers() {
		if m := refFinContainerRe.FindStringSubmatch(string(container)); m != nil {
			totals[m[1]] += len(doc.ItemKeys(container))
		}
	}
	var out []core.Issue
	for person, n := range totals {
		if n > maxReferencesFinancieres {
			out = append(out, ruleIssue(ruleReferencesFinMax20, core.Path(person+".referencesFinancieres"),
				core.SeverityBlocking,
				fmt.Sprintf("At most %d financial references per person (%d present).", maxReferencesFinancieres, n),
				"max", strconv.Itoa(maxReferencesFinancieres), "present", strconv.Itoa(n)))
		}
	}
	return out
}

func sousCategorieAutre(spec *core.Spec, doc *core.Document) []core.Issue {
	var out []core.Issue
	for p, v := range doc.Values {
		if v.Value == "" || !strings.HasSuffix(p.Template(), sousCategorieAutreSuffix) {
			continue
		}
		sub := doc.Values[p.Parent().Child(sousCategorieSiblingField)].Value
		if !slices.Contains(spec.CodeLists[relationsCodeList].OtherCodes, sub) {
			out = append(out, ruleIssue(ruleSousCategorieAutre, p, core.SeverityWarning,
				"The detail applies only when the chosen relation sub-category is \"other\".",
				"value", sub))
		}
	}
	return out
}

// dirigeantBeneficiaireDistinct blocks a director declared as not the beneficial
// owner who is nevertheless referenced as the beneficial owner: the rules of the
// spec only check that one beneficial owner is given, they cannot compare two
// references.
func dirigeantBeneficiaireDistinct(doc *core.Document) []core.Issue {
	var out []core.Issue
	for _, pk := range doc.ItemKeys("personnes.pm") {
		container := core.Path("personnes.pm[" + pk + "]").Child("dirigeant")
		for _, dk := range doc.ItemKeys(container) {
			d := core.Path(string(container) + "[" + dk + "]")
			if doc.Values[d.Child("beneficiaireEffectif")].Value != "Non" {
				continue
			}
			owner := doc.Values[d.Child("idBeneficiaireEffectif")].Value
			if owner != "" && owner == doc.Values[d.Child("idPersonne")].Value {
				out = append(out, ruleIssue(ruleDirigeantBenefDistinct, d.Child("idBeneficiaireEffectif"), core.SeverityBlocking,
					"The director is declared as not the beneficial owner: the beneficial owner cannot be the director.",
					"value", owner))
			}
		}
	}
	return out
}

// lienAutrePersonneDistincte blocks a link to an already declared person that
// targets the person holding the link.
func lienAutrePersonneDistincte(doc *core.Document) []core.Issue {
	var out []core.Issue
	for _, kind := range []string{"pp", "pm", "pnc"} {
		for _, k := range doc.ItemKeys(core.Path("personnes." + kind)) {
			person := core.Path("personnes." + kind + "[" + k + "]")
			container := person.Child("lienAutrePersonneDejaDeclaree")
			for _, lk := range doc.ItemKeys(container) {
				target := core.Path(string(container) + "[" + lk + "]").Child("idPersonne")
				if doc.Values[target].Value == string(person) {
					out = append(out, ruleIssue(ruleLienAutrePersonne, target, core.SeverityBlocking,
						"A person cannot be linked to themselves: the link must target another declared person.",
						"value", string(person)))
				}
			}
		}
	}
	return out
}

// periodeOperationsOrdonnee blocks an operations period that ends before it
// starts. Dates are stored YYYY-MM-DD, so they compare as strings; a malformed
// date is reported by the generic validator.
func periodeOperationsOrdonnee(doc *core.Document) []core.Issue {
	const start, end core.Path = "operations.dateDebut", "operations.dateFin"
	from, to := doc.Values[start].Value, doc.Values[end].Value
	if _, err := time.Parse(time.DateOnly, from); err != nil {
		return nil
	}
	if _, err := time.Parse(time.DateOnly, to); err != nil {
		return nil
	}
	if to >= from {
		return nil
	}
	return []core.Issue{ruleIssue(rulePeriodeOperations, end, core.SeverityBlocking,
		"The operations period ends before it starts: dateFin must not be earlier than dateDebut.",
		"start", from, "end", to)}
}

func montantHorsEuro(doc *core.Document) []core.Issue {
	const container core.Path = "work.transactions"
	var out []core.Issue
	for _, k := range doc.ItemKeys(container) {
		tx := core.Path(string(container) + "[" + k + "]")
		if devise := doc.Values[tx.Child("devise")].Value; devise != "" && devise != "EUR" {
			out = append(out, core.Issue{
				Path: tx.Child("montant"), Code: codeMontantHorsEuro, Severity: core.SeverityBlocking,
				Message: "The amount must be in euros: convert before export (entered currency: " + devise + ").",
				Params:  core.Params("currency", devise),
			})
		}
	}
	return out
}
