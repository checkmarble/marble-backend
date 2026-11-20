package engine

import (
	"fmt"
	"maps"
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
func missingCodeQuestions(env missingCodeEnv, doc *core.Document, declarationId uuid.UUID, skip map[string]bool) []models.SarQuestion {
	format := env.format
	spec := format.Spec()
	// skip holds the paths that already have a question (open or closed, a
	// replaced one included) or an open finding: never asked twice.
	asked := maps.Clone(skip)
	if asked == nil {
		asked = map[string]bool{}
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
		q := models.SarQuestion{
			Id: uuid.New(), DeclarationId: declarationId, Kind: models.SarQuestionKindQuestion,
			Paths: []string{string(issue.Path)}, Status: models.SarQuestionStatusOpen,
		}
		cands := env.candidates(doc, issue.Path, node.CodeList)
		var opts []core.QuestionOption
		switch {
		case len(cands) == 0:
			// Nothing in the data: today's question, with the unknown code.
			opts = []core.QuestionOption{{
				Label: label,
				Fills: []core.Fill{{Path: issue.Path, Value: list.UnknownCode, Origin: core.OriginAnalyst}},
			}}
			q.Text = fmt.Sprintf("valeur manquante : %s (%s) — aucune donnée de la sélection ne l'établit ; indiquez la valeur, ou « %s » si elle est réellement inconnue",
				name, issue.Path, label)
		default:
			// The data hold a value: never offer the unknown code. One option per
			// distinct code the candidates map to, checked like a fill; a value that
			// maps to no code is cited, the analyst answers with it.
			opts = env.candidateOptions(doc, issue.Path, node, list, cands)
			q.Text = fmt.Sprintf("valeur manquante : %s (%s) — %s ; confirmez ou indiquez la valeur",
				name, issue.Path, candidatesText(cands))
		}
		if len(opts) > 0 {
			rawOpts, err := marshalOptions(opts)
			if err != nil {
				continue
			}
			q.Options = rawOpts
		}
		asked[string(issue.Path)] = true
		out = append(out, q)
	}
	slices.SortStableFunc(out, func(a, b models.SarQuestion) int { return strings.Compare(a.Paths[0], b.Paths[0]) })
	return out
}

// missingCodeEnv is what the search of a value in the data needs: the format
// (its code lists and its optional core.CodeSourceMapper), the frozen selection
// and the data model of the selection (for the semantic types of its columns).
type missingCodeEnv struct {
	format    core.Format
	selection []models.SarSelectedObject
	dataModel models.DataModel
}

// codeCandidate is a value found in a source object of the item that can feed
// the code list: the cited column, its raw value and, when it maps to a code
// of the list, that code.
type codeCandidate struct {
	ref    core.ObjectRef
	field  string
	raw    string
	code   string
	mapped bool
}

// candidates looks, in the source objects of the item that holds path (the
// nearest ancestor item with sources), for the columns whose semantic type
// feeds the code list (core.CodeSourceMapper). A format without the capability,
// a column whose semantic type the data model does not give, or an item without
// source object yields none: the question then is today's one.
func (e missingCodeEnv) candidates(doc *core.Document, path core.Path, codeList string) []codeCandidate {
	mapper, ok := e.format.(core.CodeSourceMapper)
	if !ok {
		return nil
	}
	semantics := mapper.CodeListSemantics(codeList)
	if len(semantics) == 0 {
		return nil
	}
	var refs []core.ObjectRef
	items := ancestorItems(path)
	for i := len(items) - 1; i >= 0 && len(refs) == 0; i-- {
		refs = doc.ItemSources[items[i]]
	}
	sc := newScope(e.selection)
	var out []codeCandidate
	for _, ref := range refs {
		obj, ok := sc.get(ref.Table, ref.ObjectID)
		if !ok {
			continue
		}
		for _, field := range slices.Sorted(maps.Keys(obj.Snapshot)) {
			sem := core.FieldSemanticType(e.dataModel, ref.Table, field)
			if !slices.Contains(semantics, sem) {
				continue
			}
			raw, ok := valueString(obj.Snapshot[field])
			if !ok || strings.TrimSpace(raw) == "" {
				continue
			}
			c := codeCandidate{ref: ref, field: field, raw: raw}
			c.code, c.mapped = mapper.CodeFromValue(codeList, sem, raw)
			out = append(out, c)
		}
	}
	return out
}

// candidateOptions builds one option per distinct code the candidates map to
// (the first candidate is its source), each checked like a fill: the value
// format, the proof of the source, the field. An option that does not pass is
// not offered.
func (e missingCodeEnv) candidateOptions(doc *core.Document, path core.Path, node *core.Node,
	list core.CodeList, cands []codeCandidate,
) []core.QuestionOption {
	var opts []core.QuestionOption
	done := map[string]bool{}
	for _, c := range cands {
		if !c.mapped || done[c.code] {
			continue
		}
		done[c.code] = true
		tc := core.ToolContext{Format: e.format, Doc: doc, Selection: e.selection, DataModel: e.dataModel}
		chk := newChecker(tc, core.TaskSpec{Inputs: []core.ObjectRef{c.ref}}, uuid.Nil, []core.Path{path})
		fills, _ := chk.optionFills([]wireFill{{
			Path: flexString(path), Value: flexString(c.code),
			Source:        &wireSource{Table: flexString(c.ref.Table), ObjectID: flexString(c.ref.ObjectID), Field: flexString(c.field), RawValue: flexString(c.raw)},
			Justification: justification(c), Confidence: confidenceOf(core.ConfidenceMedium),
		}}, []core.Path{path}, doc)
		if len(fills) == 0 {
			continue
		}
		label := c.code
		for _, code := range list.Codes {
			if code.Value == c.code && code.Label != "" && code.Label != c.code {
				label = code.Label + " (" + c.code + ")"
			}
		}
		opts = append(opts, core.QuestionOption{Label: label, Fills: fills})
	}
	return opts
}

func justification(c codeCandidate) *flexString {
	s := flexString(fmt.Sprintf("valeur lue dans %s.%s", c.ref.Table, c.field))
	return &s
}

func confidenceOf(c core.Confidence) *flexString {
	s := flexString(c)
	return &s
}

// candidatesText cites the values found, at most three.
func candidatesText(cands []codeCandidate) string {
	var parts []string
	for i, c := range cands {
		if i == 3 {
			parts = append(parts, "…")
			break
		}
		parts = append(parts, fmt.Sprintf("%s (%s/%s, champ %s)", shorten(c.raw, 60), c.ref.Table, c.ref.ObjectID, c.field))
	}
	return "la sélection contient " + strings.Join(parts, ", ")
}
