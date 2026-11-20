package engine

import (
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// maxMissingCodeQuestions bounds the questions one generation adds for fields
// left empty (a hundred transactions all missing a country would otherwise
// bury the analyst; the issues of the validator stay listed anyway).
const maxMissingCodeQuestions = 30

// missingCodeQuestions asks the analyst about the required coded fields the
// generation left empty, when the code list of the field has an unknown code
// (core.CodeList.UnknownCode): one question per field, with one option that
// fills the unknown code (the analyst can still type a value). Without it the
// export only blocks on a field nothing in the data establishes.
//
// A field is required when the format's validator reports it missing (required,
// or the `then` of a required_if rule) at the path itself. Only fields a task
// may fill (copied nature, never a setting, computed or analyst field) are
// asked about, and only when nothing is stored at the path: a value the analyst
// cleared by hand is left alone. A field that already has an open question (the
// unproven-code question, a question of the model) is not asked twice. The
// severity follows the field, as for any other question: blocking when the
// field is mandatory in the format, else a warning (see buildTodo).
func missingCodeQuestions(format core.Format, doc *core.Document, declarationId uuid.UUID, open []models.SarQuestion) []models.SarQuestion {
	spec := format.Spec()
	asked := map[string]bool{}
	for _, q := range open {
		if q.Kind != models.SarQuestionKindQuestion {
			continue
		}
		for _, p := range q.Paths {
			asked[p] = true
		}
	}
	var out []models.SarQuestion
	for _, issue := range format.Validate(doc) {
		if issue.Code != core.CodeRequired && !strings.HasPrefix(issue.Code, "rule:") {
			continue
		}
		if len(out) >= maxMissingCodeQuestions || asked[string(issue.Path)] {
			continue
		}
		node := spec.NodeAt(issue.Path.Template())
		if node == nil || node.Kind != core.NodeField || node.Type != core.TypeCode || node.EffectiveNature() != core.NatureCopied {
			continue
		}
		if cur, ok := doc.Values[issue.Path]; ok && (cur.Value != "" || cur.Origin != "") {
			continue
		}
		list, ok := spec.CodeLists[node.CodeList]
		if !ok || list.UnknownCode == "" {
			continue
		}
		label := list.UnknownCode
		for _, c := range list.Codes {
			if c.Value == list.UnknownCode && c.Label != "" {
				label = c.Label
			}
		}
		name := node.Label
		if name == "" {
			name = node.Name
		}
		opts := []core.QuestionOption{{
			Label: label,
			Fills: []core.Fill{{Path: issue.Path, Value: list.UnknownCode, Origin: core.OriginAnalyst}},
		}}
		rawOpts, err := marshalOptions(opts)
		if err != nil {
			continue
		}
		asked[string(issue.Path)] = true
		out = append(out, models.SarQuestion{
			Id: uuid.New(), DeclarationId: declarationId, Kind: models.SarQuestionKindQuestion,
			Paths: []string{string(issue.Path)}, Status: models.SarQuestionStatusOpen, Options: rawOpts,
			Text: fmt.Sprintf("valeur manquante : %s (%s) — aucune donnée de la sélection ne l'établit ; indiquez la valeur, ou « %s » si elle est réellement inconnue",
				name, issue.Path, label),
		})
	}
	slices.SortStableFunc(out, func(a, b models.SarQuestion) int { return strings.Compare(a.Paths[0], b.Paths[0]) })
	return out
}
