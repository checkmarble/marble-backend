package engine

import (
	"fmt"
	"strings"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// The corrections sent back to the model are English, and so are the messages
// of the format core; the question shown to the analyst when the correction
// budget is spent must be French. Two entry points:
//   - frenchIssueReason maps a core issue by its stable code (and params), the
//     path to use whenever the issue itself is at hand;
//   - frenchReason maps a correction string (checker errors and the English
//     messages of the core) by its fixed opening. A text that matches nothing
//     gets a generic French phrase: the analyst never reads English.
var frenchReasons = []struct{ prefix, french string }{
	{"invalid path", "chemin invalide"},
	{"path outside the part", "chemin hors de la partie confiée à cette tâche"},
	{"source without field", "source sans champ"},
	{"table and object_id go together", "la table et l'identifiant de l'objet source vont ensemble"},
	{"the element has several source objects", "plusieurs objets sources possibles : l'objet source n'est pas précisé"},
	{"this element has no default source object", "l'objet source n'est pas précisé"},
	{"the source object ", "l'objet source ne fait pas partie de la sélection"},
	{"raw_value does not match", "la valeur brute citée ne correspond pas à celle de l'objet source"},
	{"the value does not match the cited source value", "la valeur ne correspond pas à celle de la source citée"},
	{"justify the conversion from cents", "la conversion des centimes en euros n'est pas justifiée"},
	{"justify the correspondence", "la correspondance avec la valeur de la source n'est pas justifiée"},
	{"the answer is not JSON", "réponse illisible"},
	{"write the code ", "le code de la liste doit être écrit tel quel, et non son libellé"},
	// English messages of the core (see core/ISSUE_CODES.md).
	{"empty path", "chemin invalide"},
	{"unknown field in the format", "champ inconnu du format"},
	{"no value at this path", "aucune valeur à ce chemin"},
	{"cannot clear a value", "impossible de vider la valeur d'un élément qui n'existe pas encore"},
	{"this identifier is generated", "identifiant généré à l'export : aucune valeur à saisir"},
	{"this path designates a group", "ce chemin désigne un groupe, pas un champ"},
	{"the item ", "l'élément n'existe pas"},
	{"provisional key", "clé provisoire sur un élément non répétable"},
	{"the origin of the value", "origine de la valeur manquante"},
	{"unknown origin", "origine de la valeur inconnue"},
	{"unknown confidence", "niveau de confiance inconnu"},
	{"reference to an unknown item", "référence vers un élément non créé dans la même réponse"},
	{"reference to an item of a type", "référence vers un élément d'un type non autorisé"},
	{"the reference targets a missing item", "référence vers un élément qui n'existe pas"},
	{"this field cannot be filled by the AI", "champ que l'IA ne peut pas remplir"},
	{"a copied value must cite", "une valeur recopiée doit citer au moins une source"},
	{"a judgment needs", "un jugement doit être justifié et accompagné d'un niveau de confiance"},
	{"source outside the scope", "source hors du périmètre autorisé pour ce champ"},
	{"character not allowed", "caractère non autorisé"},
	{"expected value: true or false", "valeur attendue : vrai ou faux"},
	{"invalid date", "date invalide (AAAA-MM-JJ attendu)"},
	{"integer expected", "nombre entier attendu"},
	{"decimal number expected", "nombre décimal attendu"},
	{"invalid value format", "format de valeur non respecté"},
	{"unreadable format pattern", "motif du format illisible"},
	{"at most ", "valeur trop longue ou nombre maximal atteint"},
	{"at least ", "valeur trop courte ou nombre minimal non atteint"},
	{"unknown code", "code absent de la liste"},
	{"unknown code list", "code absent de la liste"},
	{"not a code of ", "code absent de la liste, à vérifier"},
	{"code list not published", "liste de codes non publiée, valeur non vérifiée"},
	{"required value", "valeur obligatoire"},
	{"rule not satisfied", "règle du format non respectée"},
	{"unknown rule kind", "règle du format inconnue"},
	{"unknown condition operator", "règle du format inconnue"},
}

// genericFrenchReason is the phrase for a refusal that cannot be told more
// precisely (an unknown code, a rule description).
const genericFrenchReason = "valeur refusée par le format"

// frenchIssueCodes gives the short French phrase of each issue code of the
// core (core/ISSUE_CODES.md); the ones with params are built by
// frenchIssueReason.
var frenchIssueCodes = map[string]string{
	core.CodeUnknownPath:          "chemin invalide",
	core.CodeUnknownItem:          "l'élément n'existe pas",
	core.CodeExportIDValue:        "identifiant généré à l'export : aucune valeur à saisir",
	core.CodeNotAField:            "ce chemin désigne un groupe, pas un champ",
	core.CodeBadChar:              "caractère non autorisé",
	core.CodeBadBoolean:           "valeur attendue : vrai ou faux",
	core.CodeBadDate:              "date invalide (AAAA-MM-JJ attendu)",
	core.CodeBadInteger:           "nombre entier attendu",
	core.CodeBadDecimal:           "nombre décimal attendu",
	core.CodeMaxLength:            "valeur trop longue",
	core.CodeMinLength:            "valeur trop courte",
	core.CodePattern:              "format de valeur non respecté",
	core.CodeBadPattern:           "motif du format illisible",
	core.CodeUnknownCode:          "code absent de la liste",
	core.CodeFreeTextCode:         "code absent de la liste, saisi en texte libre à vérifier",
	core.CodeCodeListUnavailable:  "liste de codes non publiée, valeur non vérifiée",
	core.CodeBadRef:               "référence vers un élément d'un type non autorisé",
	core.CodeDanglingRef:          "référence vers un élément qui n'existe pas",
	core.CodeRequired:             "valeur obligatoire",
	core.CodeTooFewItems:          "nombre minimal d'éléments non atteint",
	core.CodeTooManyItems:         "nombre maximal d'éléments atteint",
	core.CodeUnknownRule:          "règle du format inconnue",
	core.CodeNotFound:             "aucune valeur à ce chemin",
	core.CodeEmptyNewItem:         "impossible de vider la valeur d'un élément qui n'existe pas encore",
	core.CodeBadProvisionalKey:    "clé provisoire sur un élément non répétable",
	core.CodeOutOfScope:           "chemin hors de la partie confiée à cette tâche",
	core.CodeMissingOrigin:        "origine de la valeur manquante",
	core.CodeUnknownOrigin:        "origine de la valeur inconnue",
	core.CodeBadConfidence:        "niveau de confiance inconnu",
	core.CodeNotWritableByLLM:     "champ que l'IA ne peut pas remplir",
	core.CodeMissingSource:        "une valeur recopiée doit citer au moins une source",
	core.CodeMissingJustification: "un jugement doit être justifié et accompagné d'un niveau de confiance",
	core.CodeSourceOutOfScope:     "source hors du périmètre autorisé pour ce champ",
	core.CodeOriginNotAllowed:     "cette origine ne peut pas écrire ce champ (réglage de l'organisation)",
}

// frenchIssueReason is the French reason of a core issue, from its code and
// params, never from its English message. A rule issue (rule:<id>) is generic,
// the rule id is not shown; an unknown code gets the generic phrase.
func frenchIssueReason(is core.Issue) string {
	if strings.HasPrefix(is.Code, "rule:") {
		return "règle du format non respectée"
	}
	fr, ok := frenchIssueCodes[is.Code]
	if !ok {
		return genericFrenchReason
	}
	p := is.Params
	switch is.Code {
	case core.CodeMaxLength:
		if p["max"] != "" {
			return fmt.Sprintf("valeur trop longue (%s %s au maximum)", p["max"], frenchUnit(p["unit"]))
		}
	case core.CodeMinLength:
		if p["min"] != "" {
			return fmt.Sprintf("valeur trop courte (%s %s au minimum)", p["min"], frenchUnit(p["unit"]))
		}
	case core.CodeBadDecimal:
		if p["max_fraction_digits"] != "" {
			return fmt.Sprintf("trop de décimales (%s au maximum)", p["max_fraction_digits"])
		}
	case core.CodeTooManyItems:
		if p["max"] != "" {
			return fmt.Sprintf("nombre maximal d'éléments atteint (%s)", p["max"])
		}
	case core.CodeTooFewItems:
		if p["min"] != "" {
			return fmt.Sprintf("nombre minimal d'éléments non atteint (%s)", p["min"])
		}
	}
	return fr
}

func frenchUnit(unit string) string {
	if unit == "digits" {
		return "chiffres"
	}
	return "caractères"
}

// refHint is appended to the correction of a reference error for the model.
const refHint = " (values that reference an element"

func frenchReason(reason string) string {
	if i := strings.Index(reason, refHint); i >= 0 {
		reason = reason[:i]
	}
	for _, r := range frenchReasons {
		if strings.HasPrefix(reason, r.prefix) {
			return r.french
		}
	}
	return genericFrenchReason
}
