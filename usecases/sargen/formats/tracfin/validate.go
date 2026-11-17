package tracfin

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

const (
	ruleCadreLegalPrincipal   = "cadre_legal_principal"
	rulePersonnesAuMoinsUne   = "personnes_au_moins_une"
	ruleReferencesFinMax20    = "references_financieres_max_20"
	ruleSousCategorieAutre    = "sous_categorie_relation_autre"
	codeMontantHorsEuro       = "montant_hors_euro"
	maxReferencesFinancieres  = 20
	relationsCodeList         = "liste_ref_relations.csv"
	sousCategorieAutreSuffix  = ".typeRelation[].sousCategorieRelationAutre"
	sousCategorieSiblingField = "sousCategorieRelation"
)

// HandRuleIDs lists the hand-written rules implemented in this file. A test
// checks it against `hand_rules` in constraints.json (nothing missing, nothing
// extra).
func HandRuleIDs() []string {
	return []string{ruleCadreLegalPrincipal, rulePersonnesAuMoinsUne, ruleReferencesFinMax20, ruleSousCategorieAutre}
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
	issues = append(issues, montantHorsEuro(doc)...)
	tooManyOperationsIssue(issues, doc)
	core.SortIssues(issues)
	return issues
}

func ruleIssue(id string, path core.Path, sev core.Severity, msg string) core.Issue {
	return core.Issue{Path: path, Code: "rule:" + id, Message: msg, Severity: sev, RuleID: id}
}

func cadreLegalPrincipal(doc *core.Document) []core.Issue {
	const container core.Path = "context.natureSoupcon.cadreLegal"
	keys := doc.Items[container]
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
		"Le cadre légal doit contenir au moins un des codes CL01, CL02 ou CL03.")}
}

func personnesAuMoinsUne(doc *core.Document) []core.Issue {
	present := false
	for p, v := range doc.Values {
		if v.Value != "" && p.HasPrefix("personnes") {
			present = true
		}
	}
	for p, keys := range doc.Items {
		if len(keys) > 0 && p.HasPrefix("personnes") {
			present = true
		}
	}
	if !present {
		return nil
	}
	if len(doc.Items["personnes.pp"])+len(doc.Items["personnes.pm"])+len(doc.Items["personnes.pnc"]) > 0 {
		return nil
	}
	return []core.Issue{ruleIssue(rulePersonnesAuMoinsUne, "personnes", core.SeverityBlocking,
		"La section personnes doit contenir au moins une personne (physique, morale ou non caractérisée).")}
}

func referencesFinancieresMax(doc *core.Document) []core.Issue {
	totals := map[string]int{}
	for container, keys := range doc.Items {
		if m := refFinContainerRe.FindStringSubmatch(string(container)); m != nil {
			totals[m[1]] += len(keys)
		}
	}
	var out []core.Issue
	for person, n := range totals {
		if n > maxReferencesFinancieres {
			out = append(out, ruleIssue(ruleReferencesFinMax20, core.Path(person+".referencesFinancieres"),
				core.SeverityBlocking,
				fmt.Sprintf("%d références financières au maximum par personne (%d saisies).", maxReferencesFinancieres, n)))
		}
	}
	return out
}

func isAutreLabel(label string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(label)), "autre")
}

func sousCategorieAutre(spec *core.Spec, doc *core.Document) []core.Issue {
	var out []core.Issue
	for p, v := range doc.Values {
		if v.Value == "" || !strings.HasSuffix(p.Template(), sousCategorieAutreSuffix) {
			continue
		}
		sub := doc.Values[p.Parent().Child(sousCategorieSiblingField)].Value
		autre := false
		for _, c := range spec.CodeLists[relationsCodeList].Codes {
			if c.Value == sub && isAutreLabel(c.Label) {
				autre = true
			}
		}
		if !autre {
			out = append(out, ruleIssue(ruleSousCategorieAutre, p, core.SeverityWarning,
				"La précision n'est à remplir que si la sous-catégorie de relation choisie est « autre »."))
		}
	}
	return out
}

func montantHorsEuro(doc *core.Document) []core.Issue {
	const container core.Path = "work.transactions"
	var out []core.Issue
	for _, k := range doc.Items[container] {
		tx := core.Path(string(container) + "[" + k + "]")
		if devise := doc.Values[tx.Child("devise")].Value; devise != "" && devise != "EUR" {
			out = append(out, core.Issue{
				Path: tx.Child("montant"), Code: codeMontantHorsEuro, Severity: core.SeverityBlocking,
				Message: "Le montant doit être en euros : convertir avant l'export (devise saisie : " + devise + ").",
			})
		}
	}
	return out
}
