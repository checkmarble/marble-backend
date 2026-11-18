package engine

import "strings"

// The corrections sent back to the model are English; the question shown to the
// analyst when the correction budget is spent must be French. frenchReason maps
// each kind of checker error (by its fixed opening) to a short French phrase.
// A message that matches none is a validation issue of the format core, already
// written in French for the analyst, and passes through unchanged.
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
	return reason
}
