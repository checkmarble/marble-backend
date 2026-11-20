package engine

import (
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // Paris day of timestamps, whatever the host's zoneinfo
	"unicode"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// flexString reads a JSON string, tolerating numbers and booleans (kept as
// their text) and null (empty).
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	switch {
	case s == "null":
		*f = ""
	case strings.HasPrefix(s, `"`):
		var str string
		if err := json.Unmarshal(b, &str); err != nil {
			return err
		}
		*f = flexString(str)
	default:
		*f = flexString(s)
	}
	return nil
}

const indexMarker = "\x00idx:"

type wireSource struct {
	Table    flexString `json:"table"`
	ObjectID flexString `json:"object_id"` //nolint:tagliatelle
	Field    flexString `json:"field"`
	RawValue flexString `json:"raw_value"` //nolint:tagliatelle
}

type wireFill struct {
	Path          flexString  `json:"path"`
	Value         flexString  `json:"value"`
	Source        *wireSource `json:"source"`
	RuleID        *flexString `json:"rule_id"` //nolint:tagliatelle
	Justification *flexString `json:"justification"`
	Confidence    *flexString `json:"confidence"`
}

type wireOption struct {
	Label flexString `json:"label"`
	Fills []wireFill `json:"fills"`
}

type wireQuestion struct {
	Paths   []flexString `json:"paths"`
	Text    flexString   `json:"text"`
	Options []wireOption `json:"options"`
}

type wireOutput struct {
	Fills     []wireFill     `json:"fills"`
	Questions []wireQuestion `json:"questions"`
}

type wireFinding struct {
	Paths    []flexString `json:"paths"`
	Severity flexString   `json:"severity"`
	Message  flexString   `json:"message"`
	// Options are proposals with fills; ContestsQuestionID is the open question
	// this finding replaces. Both optional.
	Options            []wireOption `json:"options"`
	ContestsQuestionID *flexString  `json:"contests_question_id"` //nolint:tagliatelle
}

// checker applies the output control to the final answers of one task run.
type checker struct {
	tc      core.ToolContext
	spec    core.TaskSpec
	taskID  uuid.UUID
	allowed []core.Path
	sc      scope
	// ruleIDs are the memory rules the LLM may cite; any other rule_id is
	// dropped.
	ruleIDs map[string]bool
	// before holds the issues of the document before the task's fills.
	before map[ruleKey]bool
}

func newChecker(tc core.ToolContext, spec core.TaskSpec, taskID uuid.UUID, allowed []core.Path) *checker {
	rules := make(map[string]bool, len(tc.Rules))
	for _, r := range tc.Rules {
		rules[r.Id.String()] = true
	}
	return &checker{tc: tc, spec: spec, taskID: taskID, allowed: allowed, sc: newScope(tc.Selection), ruleIDs: rules}
}

// checkResult is the outcome of the control of one final answer.
type checkResult struct {
	Fills     []core.Fill
	Questions []core.QuestionDraft
	// Corrections are the form errors to send back to the LLM.
	Corrections []string
	// Soft are the corrections that stem from a warning-level format rule: when
	// the correction budget runs out on them, the fill is dropped silently
	// instead of becoming a question.
	Soft map[string]bool
	// Notes are control remarks kept in the trace only.
	Notes []string
}

// neverSendBack lists the issue codes that are completeness matters of the
// whole document (checked by the validator, shown to the analyst), never errors
// the LLM could fix by rewriting its answer.
func neverSendBack(code string) bool {
	return code == core.CodeRequired || code == core.CodeTooFewItems
}

// pendingFill is a fill of an answer that passed the form checks, with its
// position in the answer.
type pendingFill struct {
	idx  int
	fill core.Fill
}

// checkFills controls a final answer of a fill task.
func (c *checker) checkFills(final json.RawMessage) checkResult {
	var res checkResult
	var out wireOutput
	if err := json.Unmarshal(final, &out); err != nil {
		res.Corrections = append(res.Corrections, "the answer is not JSON conforming to the requested schema: "+err.Error())
		return res
	}

	// Last wins when a path appears several times in one answer.
	last := map[core.Path]int{}
	for i, w := range out.Fills {
		last[core.Path(w.Path)] = i
	}
	var (
		good          []pendingFill
		unprovenFills []core.Fill
	)
	for i, w := range out.Fills {
		path := core.Path(w.Path)
		if last[path] != i {
			res.Notes = append(res.Notes, fmt.Sprintf("fills[%d] %s : duplicate path in the answer, the last value is kept", i, path))
			continue
		}
		node := c.tc.Format.Spec().NodeAt(path.Template())
		if w.Value == "" && node != nil && node.Kind == core.NodeField && !node.ExportID {
			// The LLM never clears a value. An empty required field is left to
			// the analyst, never sent back as an error.
			if node.Min >= 1 && c.pathAllowed(path) {
				res.Questions = append(res.Questions, core.QuestionDraft{
					Paths: []core.Path{path},
					Text:  fmt.Sprintf("valeur absente : le champ %s est obligatoire et n'a pas pu être renseigné automatiquement", path),
				})
			}
			res.Notes = append(res.Notes, fmt.Sprintf("fills[%d] %s : empty value ignored", i, path))
			continue
		}
		f, msg, unproven := c.toFill(w, true)
		if msg != "" {
			res.Corrections = append(res.Corrections, fmt.Sprintf("fills[%d] %s : %s", i, path, msg))
			continue
		}
		if unproven {
			// A strict code (e.g. "Non") is not established: it is not applied,
			// the analyst chooses.
			unprovenFills = append(unprovenFills, f)
			res.Notes = append(res.Notes, fmt.Sprintf("fills[%d] %s : strict code without a strict proof, turned into a question", i, path))
			continue
		}
		good = append(good, pendingFill{i, f})
	}

	// References to items created in the same answer ([+a]) are applied after
	// the other values, so that the item exists when they are checked.
	slices.SortStableFunc(good, func(x, y pendingFill) int {
		return boolToInt(c.isRefFill(x.fill)) - boolToInt(c.isRefFill(y.fill))
	})

	// Dry run on a copy of the document: natures, sources, scope, values. Each
	// fill carries its position in a marker so that a rejection is matched to
	// its input by index (the clone is discarded: the markers never leak).
	clone := c.tc.Doc.Clone()
	fills := make([]core.Fill, len(good))
	for i, g := range good {
		f := g.fill
		f.RuleIDs = append(slices.Clone(f.RuleIDs), indexMarker+strconv.Itoa(i))
		fills[i] = f
	}
	applied := core.ApplyFills(c.tc.Format.Spec(), clone, fills, c.applyOptions(clone))
	rejectedIdx := map[int]core.Issue{}
	for _, rj := range applied.Rejected {
		if n := len(rj.Fill.RuleIDs); n > 0 && strings.HasPrefix(rj.Fill.RuleIDs[n-1], indexMarker) {
			if i, err := strconv.Atoi(strings.TrimPrefix(rj.Fill.RuleIDs[n-1], indexMarker)); err == nil {
				rejectedIdx[i] = rj.Issue
			}
		}
	}
	withoutFill := func(skip int) *core.Document {
		d := c.tc.Doc.Clone()
		rest := make([]core.Fill, 0, len(fills))
		for i, f := range fills {
			if i != skip {
				rest = append(rest, f)
			}
		}
		core.ApplyFills(c.tc.Format.Spec(), d, rest, c.applyOptions(d))
		return d
	}
	ruleIssues := c.newRuleIssues(clone, good, rejectedIdx, withoutFill)
	for i, g := range good {
		issue, bad := rejectedIdx[i]
		if !bad {
			if rule, hit := ruleIssues[g.fill.Path]; hit {
				msg := fmt.Sprintf("fills[%d] %s : %s", g.idx, g.fill.Path, rule.Message)
				res.Corrections = append(res.Corrections, msg)
				if rule.Severity != core.SeverityBlocking {
					if res.Soft == nil {
						res.Soft = map[string]bool{}
					}
					res.Soft[msg] = true
				}
				continue
			}
			res.Fills = append(res.Fills, g.fill)
			continue
		}
		if neverSendBack(issue.Code) {
			res.Questions = append(res.Questions, unverifiable(g.fill, issue.Message))
			continue
		}
		msg := issue.Message
		if issue.Code == core.CodeBadRef || issue.Code == core.CodeDanglingRef {
			msg += refHint + " are applied after all the others of the answer: create the targeted element in the same answer, with the same provisional key, or cite an existing element)"
		}
		res.Corrections = append(res.Corrections, fmt.Sprintf("fills[%d] %s : %s", g.idx, g.fill.Path, msg))
	}

	for _, f := range unprovenFills {
		res.Questions = append(res.Questions, c.unprovenCodeQuestion(f, clone))
	}
	c.checkQuestions(&res, out.Questions, clone)
	return res
}

// ruleKey identifies a validation issue of the document before the fills.
type ruleKey struct {
	code string
	path core.Path
}

// issuesBefore are the issues of the document the task starts from, computed
// once per checker (the document of a checker never changes).
func (c *checker) issuesBefore() map[ruleKey]bool {
	if c.before == nil {
		c.before = map[ruleKey]bool{}
		for _, is := range c.tc.Format.Validate(c.tc.Doc) {
			c.before[ruleKey{is.Code, is.Path}] = true
		}
	}
	return c.before
}

// newRuleIssues returns, by written path, the format rule issue (code `rule:*`,
// a warning as well as a blocking one) that the candidate fills cause: present
// when the document is validated with the fills applied (after), absent from the
// document before them, and whose path is a path one of the accepted fills
// writes or an ancestor of it (a rule on a group or on the scope of an
// exclusivity rule is raised on the group or the scope, not on the leaf). A
// `required` issue, an issue already in the document and an issue elsewhere are
// not the fills' doing and are ignored: the fill that triggers a rule is sent
// back to the model with the message of the rule.
func (c *checker) newRuleIssues(after *core.Document, good []pendingFill, rejected map[int]core.Issue,
	without func(goodIdx int) *core.Document,
) map[core.Path]core.Issue {
	var written []int
	for i := range good {
		if _, bad := rejected[i]; !bad {
			written = append(written, i)
		}
	}
	if len(written) == 0 {
		return nil
	}
	before := c.issuesBefore()
	var fresh []core.Issue
	for _, is := range c.tc.Format.Validate(after) {
		if strings.HasPrefix(is.Code, "rule:") && !before[ruleKey{is.Code, is.Path}] {
			fresh = append(fresh, is)
		}
	}
	var out map[core.Path]core.Issue
	blame := func(p core.Path, is core.Issue) {
		if out == nil {
			out = map[core.Path]core.Issue{}
		}
		if _, dup := out[p]; !dup {
			out[p] = is
		}
	}
	for _, is := range fresh {
		var under []int
		for _, i := range written {
			w := good[i].fill.Path
			switch {
			case is.Path == w:
				blame(w, is)
			case strings.HasPrefix(string(w), string(is.Path)+".") || strings.HasPrefix(string(w), string(is.Path)+"["):
				under = append(under, i)
			}
		}
		// An issue raised on an ancestor is blamed on the fills whose removal
		// makes it disappear; when none does alone (it takes several fills
		// together), on all the fills under the ancestor.
		var culprits []int
		for _, i := range under {
			stays := false
			for _, other := range c.tc.Format.Validate(without(i)) {
				if other.Code == is.Code && other.Path == is.Path {
					stays = true
					break
				}
			}
			if !stays {
				culprits = append(culprits, i)
			}
		}
		if len(culprits) == 0 {
			culprits = under
		}
		for _, i := range culprits {
			blame(good[i].fill.Path, is)
		}
	}
	return out
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// isRefFill tells whether the fill's value is a reference to another item.
func (c *checker) isRefFill(f core.Fill) bool {
	n := c.tc.Format.Spec().NodeAt(f.Path.Template())
	return n != nil && n.Kind == core.NodeField && (n.Type == core.TypeRef || n.IsItemRef(f.Value))
}

func (c *checker) applyOptions(doc *core.Document) core.ApplyOptions {
	return core.ApplyOptions{
		AllowedPrefixes: c.allowed,
		Overwrite:       core.OverwriteNonManual,
		AllowedSources:  AllowedSources(doc, c.tc.Selection, c.tc.DataModel, c.spec.Inputs),
	}
}

func (c *checker) pathAllowed(p core.Path) bool {
	return core.PathAllowed(p, c.allowed)
}

// checkQuestions keeps the model's questions: paths outside the task's part are
// dropped, an option keeps its fills only when all pass the same controls as
// direct fills (else it stays a plain label).
func (c *checker) checkQuestions(res *checkResult, qs []wireQuestion, afterFills *core.Document) {
	for qi, wq := range qs {
		q := core.QuestionDraft{Text: string(wq.Text)}
		for _, p := range wq.Paths {
			if c.pathAllowed(core.Path(p)) {
				q.Paths = append(q.Paths, core.Path(p))
			} else {
				res.Notes = append(res.Notes, fmt.Sprintf("questions[%d] : path outside the assigned part ignored: %s", qi, p))
			}
		}
		createsItem := false
		var declines []int // indexes (in the wire options) of the options with no fill at all
		kept := map[int]core.QuestionOption{}
		for oi, wo := range wq.Options {
			opt := core.QuestionOption{Label: string(wo.Label)}
			fills, note := c.optionFills(wo.Fills, q.Paths, afterFills)
			if note != "" {
				res.Notes = append(res.Notes, fmt.Sprintf("questions[%d].options[%d] : %s", qi, oi, note))
			}
			if len(fills) == 0 {
				if len(wo.Fills) == 0 {
					declines = append(declines, oi)
				} else {
					res.Notes = append(res.Notes, fmt.Sprintf("questions[%d].options[%d] : no value to write, option dropped", qi, oi))
				}
				continue
			}
			for _, f := range fills {
				createsItem = createsItem || f.Path.HasProvisionalKey()
			}
			opt.Fills = fills
			kept[oi] = opt
		}
		for _, oi := range declines {
			// An option that writes nothing cannot be chosen, unless it declines
			// the addition of a list item proposed by a sibling option.
			if createsItem {
				kept[oi] = core.QuestionOption{Label: string(wq.Options[oi].Label)}
			} else {
				res.Notes = append(res.Notes, fmt.Sprintf("questions[%d].options[%d] : no value to write, option dropped", qi, oi))
			}
		}
		for oi := range wq.Options {
			if opt, ok := kept[oi]; ok {
				q.Options = append(q.Options, opt)
			}
		}
		if len(wq.Paths) > 0 && len(q.Paths) == 0 {
			res.Notes = append(res.Notes, fmt.Sprintf("questions[%d] : no path left in the assigned part, question ignored", qi))
			continue
		}
		res.Questions = append(res.Questions, q)
	}
}

// optionFills returns the fills of an option when all pass, else none and the
// reason. An option is the analyst's choice when clicked, so no source is
// required: fills are validated with the analyst origin (the nature and source
// requirements for LLM values do not apply) and a replaceable manual value
// does not block them. Settings and computed fields are refused; a cited
// source still goes through the proof checks of toFill.
func (c *checker) optionFills(ws []wireFill, questionPaths []core.Path, doc *core.Document) ([]core.Fill, string) {
	if len(ws) == 0 {
		return nil, ""
	}
	fills := make([]core.Fill, 0, len(ws))
	for _, w := range ws {
		if w.Value == "" {
			return nil, "empty value"
		}
		// An option is the analyst's choice: the unknown-code proof rule does
		// not apply.
		f, msg, _ := c.toFill(w, false)
		if msg != "" {
			return nil, msg
		}
		if !core.PathAllowed(f.Path, questionPaths) {
			return nil, fmt.Sprintf("%s is outside the fields of the question", f.Path)
		}
		if node := c.tc.Format.Spec().NodeAt(f.Path.Template()); node != nil {
			switch node.EffectiveNature() {
			case core.NatureSetting, core.NatureComputed:
				return nil, fmt.Sprintf("%s is a setting or a computed field", f.Path)
			}
		}
		f.Origin, f.TaskID = core.OriginAnalyst, nil
		fills = append(fills, f)
	}
	clone := doc.Clone()
	opts := c.applyOptions(clone)
	opts.Overwrite = core.OverwriteAll
	res := core.ApplyFills(c.tc.Format.Spec(), clone, fills, opts)
	if len(res.Rejected) > 0 {
		return nil, fmt.Sprintf("%s : %s", res.Rejected[0].Fill.Path, res.Rejected[0].Issue.Message)
	}
	return fills, ""
}

func unverifiable(f core.Fill, reason string) core.QuestionDraft {
	v := []rune(f.Value)
	if len(v) > 200 {
		v = append(v[:200], '…')
	}
	return core.QuestionDraft{
		Paths: []core.Path{f.Path},
		Text:  fmt.Sprintf("valeur non vérifiable : %s (valeur proposée « %s ») — %s", f.Path, string(v), reason),
	}
}

// toFill converts one wire fill; the message is a form error for the LLM.
// unproven (only with strict) tells that the fill is a copied value of a field
// whose code list declares the code as strict (e.g. "Non") and that its source is
// not a strict proof: the caller must not apply it.
func (c *checker) toFill(w wireFill, strict bool) (fill core.Fill, msg string, unproven bool) {
	path := core.Path(w.Path)
	if _, err := core.ParsePath(string(path)); err != nil {
		return core.Fill{}, "invalid path: " + err.Error(), false
	}
	if !c.pathAllowed(path) {
		return core.Fill{}, "path outside the part assigned to this task; allowed paths: " + joinPaths(c.allowed), false
	}
	f := core.Fill{
		Path: path, Value: string(w.Value), Origin: core.OriginLLM, TaskID: &c.taskID,
	}
	if w.RuleID != nil && c.ruleIDs[string(*w.RuleID)] {
		f.RuleIDs = []string{string(*w.RuleID)}
	}
	if w.Justification != nil {
		f.Justification = string(*w.Justification)
	}
	if w.Confidence != nil {
		f.Confidence = core.Confidence(*w.Confidence)
	}
	node := c.tc.Format.Spec().NodeAt(path.Template())
	copied := node != nil && node.Kind == core.NodeField && node.EffectiveNature() == core.NatureCopied
	var list core.CodeList
	if copied && node.CodeList != "" {
		list = c.tc.Format.Spec().CodeLists[node.CodeList]
	}
	// The unknown code needs no source: any source given with it is dropped. A
	// code the format declares strict (e.g. "Non") needs a strict proof; the other
	// codes of the list keep the usual relation rules.
	if list.UnknownCode != "" && f.Value == list.UnknownCode {
		return f, "", false
	}
	gated := strict && slices.Contains(list.StrictCodes, f.Value)
	if w.Source == nil {
		return f, "", gated
	}
	src, smsg := c.toSource(path, *w.Source)
	if smsg != "" {
		return core.Fill{}, smsg, false
	}
	f.Sources = []core.SourceRef{src}
	if !copied {
		return f, "", false
	}
	if gated {
		return f, "", !strictCodeProof(list.Codes, f.Value, src.RawValue, c.sourceIsBoolean(src))
	}
	if list.UnknownCode != "" && strictCodeProof(list.Codes, f.Value, src.RawValue, c.sourceIsBoolean(src)) {
		return f, "", false // a strict proof needs no judgment, for any code of the list
	}
	rmsg, forceLow := relationError(node, list.Codes, f.Value, src.RawValue, f.Justification, f.Confidence)
	if rmsg != "" {
		return core.Fill{}, rmsg, false
	}
	if forceLow {
		f.Confidence = core.ConfidenceLow
	}
	return f, "", false
}

// sourceIsBoolean tells whether the cited field is a boolean in the data model:
// only then does a boolean literal prove a yes/no code (0, 1, yes or no in a
// text or numeric column prove nothing).
func (c *checker) sourceIsBoolean(src core.SourceRef) bool {
	table, ok := c.tc.DataModel.Tables[src.Object.Table]
	if !ok {
		return false
	}
	field, ok := table.Fields[src.Field]
	return ok && field.DataType == models.Bool
}

// strictCodeProof tells whether the raw value of the source proves the code
// without interpretation: it is the code itself, its label (folded like text
// proofs), one of its aliases, or, when the source field is a boolean, a boolean literal matching a
// yes/no code.
func strictCodeProof(codes []core.Code, value, raw string, boolSource bool) bool {
	if foldAlnum(value) == foldAlnum(raw) || slices.ContainsFunc(codes, func(c core.Code) bool {
		if c.Value != value {
			return false
		}
		if c.Label != "" && foldAlnum(c.Label) == foldAlnum(raw) {
			return true
		}
		return slices.ContainsFunc(c.Aliases, func(a string) bool { return foldAlnum(a) == foldAlnum(raw) })
	}) {
		return true
	}
	if !boolSource {
		return false
	}
	rb, rok := boolLiteral(raw)
	vb, vok := boolLiteral(value)
	return rok && vok && rb == vb
}

// unprovenCodeQuestion asks the analyst to choose among the codes of the list
// of a field whose proposed value was not strictly proven. Each option writes
// its code on the field.
func (c *checker) unprovenCodeQuestion(f core.Fill, doc *core.Document) core.QuestionDraft {
	node := c.tc.Format.Spec().NodeAt(f.Path.Template())
	list := c.tc.Format.Spec().CodeLists[node.CodeList]
	unknown := list.UnknownCode
	for _, code := range list.Codes {
		if code.Value == unknown && code.Label != "" {
			unknown = code.Label
		}
	}
	q := core.QuestionDraft{
		Paths: []core.Path{f.Path},
		Text: fmt.Sprintf("valeur non établie : %s — « %s » proposé sans preuve suffisante dans les données ; choisissez la valeur (« %s » si rien ne l'établit)",
			f.Path, shorten(f.Value, 100), unknown),
	}
	for _, code := range list.Codes {
		label := code.Label
		if label == "" {
			label = code.Value
		}
		opt := core.QuestionOption{Label: label}
		fills, _ := c.optionFills([]wireFill{{Path: flexString(f.Path), Value: flexString(code.Value)}}, q.Paths, doc)
		if len(fills) == 0 {
			continue // an option that writes nothing is not offered
		}
		opt.Fills = fills
		q.Options = append(q.Options, opt)
	}
	return q
}

func joinPaths(ps []core.Path) string {
	s := make([]string, len(ps))
	for i, p := range ps {
		s[i] = string(p)
	}
	return strings.Join(s, ", ")
}

// toSource resolves the compact source (table and object_id default to the
// single ItemSource of the target item) and checks the proof against the frozen
// selection.
func (c *checker) toSource(target core.Path, w wireSource) (core.SourceRef, string) {
	if w.Field == "" {
		return core.SourceRef{}, "source without field (field)"
	}
	ref := core.ObjectRef{Table: string(w.Table), ObjectID: string(w.ObjectID)}
	switch {
	case ref.Table == "" && ref.ObjectID == "":
		var msg string
		if ref, msg = c.defaultSource(target); msg != "" {
			return core.SourceRef{}, msg
		}
	case ref.Table == "" || ref.ObjectID == "":
		return core.SourceRef{}, "table and object_id go together: give both or neither"
	}
	obj, ok := c.sc.get(ref.Table, ref.ObjectID)
	if !ok {
		return core.SourceRef{}, "the source object " + ref.Table + "/" + ref.ObjectID + " is not part of the selection"
	}
	if !proves(obj, string(w.Field), string(w.RawValue)) {
		return core.SourceRef{}, fmt.Sprintf(
			"raw_value does not match the value of field %q of object %s/%s: read the object again with read_object and copy the exact value",
			w.Field, ref.Table, ref.ObjectID)
	}
	return core.SourceRef{Object: ref, Field: string(w.Field), RawValue: string(w.RawValue)}, ""
}

// defaultSource is the single ItemSource of the nearest ancestor item that has
// some.
func (c *checker) defaultSource(target core.Path) (core.ObjectRef, string) {
	items := ancestorItems(target)
	for i := len(items) - 1; i >= 0; i-- {
		refs := c.tc.Doc.ItemSources[items[i]]
		switch len(refs) {
		case 0:
			continue
		case 1:
			return refs[0], ""
		default:
			names := make([]string, len(refs))
			for j, r := range refs {
				names[j] = r.Table + "/" + r.ObjectID
			}
			return core.ObjectRef{}, "the element has several source objects (" + strings.Join(names, ", ") + "): specify table and object_id"
		}
	}
	if len(c.spec.Inputs) == 1 {
		return c.spec.Inputs[0], ""
	}
	return core.ObjectRef{}, "this element has no default source object: specify table and object_id"
}

var (
	decimalRe = regexp.MustCompile(`^-?[0-9]+([.,][0-9]+)?$`)
	integerRe = regexp.MustCompile(`^-?[0-9]+$`)
	parisZone = mustParis()
)

func mustParis() *time.Location {
	l, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		return time.FixedZone("CET", 3600)
	}
	return l
}

// proves tells whether the object's field holds raw: same text, same decimal
// number (trailing fraction zeros and comma ignored), or a timestamp whose
// Paris day is raw (a date).
func proves(o models.SarSelectedObject, field, raw string) bool {
	got, ok := valueString(o.Snapshot[field])
	if field == "object_id" && o.ObjectId != "" {
		got, ok = o.ObjectId, true
	}
	if !ok || raw == "" {
		return false
	}
	if got == raw {
		return true
	}
	if decimalRe.MatchString(got) && decimalRe.MatchString(raw) && normDecimal(got) == normDecimal(raw) {
		return true
	}
	if day, ok := parisDay(got); ok {
		return day == raw
	}
	return false
}

// parisDay returns the Paris calendar day of a timestamp. A timestamp with a
// zone is converted to Paris; a naive one (no zone) is read as Paris time.
func parisDay(s string) (string, bool) {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.In(parisZone).Format("2006-01-02"), true
	}
	for _, layout := range []string{"2006-01-02T15:04:05.999999999", "2006-01-02 15:04:05.999999999"} {
		if t, err := time.ParseInLocation(layout, s, parisZone); err == nil {
			return t.Format("2006-01-02"), true
		}
	}
	return "", false
}

func normDecimal(s string) string {
	s = strings.Replace(strings.TrimSpace(s), ",", ".", 1)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	if s == "-0" || s == "" {
		return "0"
	}
	return s
}

// checkReview controls the final answer of a review task.
func (c *checker) checkReview(final json.RawMessage) ([]core.ReviewFinding, []string) {
	var out struct {
		Findings []wireFinding `json:"findings"`
	}
	if err := json.Unmarshal(final, &out); err != nil {
		return nil, []string{"the answer is not JSON conforming to the requested schema: " + err.Error()}
	}
	var (
		findings []core.ReviewFinding
		errs     []string
	)
	for i, w := range out.Findings {
		f := core.ReviewFinding{Severity: string(w.Severity), Message: strings.TrimSpace(string(w.Message))}
		bad := false
		if f.Severity != "important" && f.Severity != "mineur" {
			errs = append(errs, fmt.Sprintf("findings[%d] : severity must be important or mineur", i))
			bad = true
		}
		if f.Message == "" {
			errs = append(errs, fmt.Sprintf("findings[%d] : empty message", i))
			bad = true
		}
		for _, p := range w.Paths {
			if _, err := core.ParsePath(string(p)); err != nil {
				errs = append(errs, fmt.Sprintf("findings[%d] : %s", i, err.Error()))
				bad = true
				continue
			}
			f.Paths = append(f.Paths, core.Path(p))
		}
		var contested *core.QuestionView
		if w.ContestsQuestionID != nil && strings.TrimSpace(string(*w.ContestsQuestionID)) != "" {
			id, err := uuid.Parse(strings.TrimSpace(string(*w.ContestsQuestionID)))
			if err != nil {
				contested = nil
			} else {
				contested = c.openQuestion(id)
			}
			if contested == nil {
				errs = append(errs, fmt.Sprintf("findings[%d] : contests_question_id is not an open question of this declaration (use an id from the open questions list, or null)", i))
				bad = true
			} else {
				f.ContestsQuestionID = &contested.Id
				// The finding replaces the question: whoever answers it must be able to
				// do what the question allowed, so its paths cover the question's.
				for _, qp := range contested.Paths {
					if !slices.Contains(f.Paths, qp) {
						errs = append(errs, fmt.Sprintf("findings[%d] : a finding that contests a question must include every path of that question in paths (missing %s)", i, qp))
						bad = true
					}
				}
			}
		}
		if len(w.Options) > 0 && len(f.Paths) == 0 && !bad {
			errs = append(errs, fmt.Sprintf("findings[%d] : options need the paths of the fields they fill", i))
			bad = true
		}
		for oi, wo := range w.Options {
			if bad {
				break
			}
			opt, msg := c.reviewOption(wo, f.Paths)
			if msg != "" {
				errs = append(errs, fmt.Sprintf("findings[%d].options[%d] : %s", i, oi, msg))
				bad = true
				break
			}
			f.Options = append(f.Options, opt)
		}
		// A finding that replaces a question can be blocking: it must be answerable,
		// through an option or through a plain field the analyst can type into.
		if !bad && contested != nil && len(f.Options) == 0 && !slices.ContainsFunc(f.Paths, c.answerable) {
			errs = append(errs, fmt.Sprintf("findings[%d] : a finding that contests a question needs at least one option, or at least one path to a plain field the analyst can answer (an element or a group cannot be answered)", i))
			bad = true
		}
		if !bad {
			findings = append(findings, f)
		}
	}
	return findings, errs
}

// openQuestion returns the open question (not a finding) with this id, as given
// to the review task.
func (c *checker) openQuestion(id uuid.UUID) *core.QuestionView {
	for i := range c.tc.OpenQuestions {
		if q := &c.tc.OpenQuestions[i]; q.Id == id && q.Kind == models.SarQuestionKindQuestion {
			return q
		}
	}
	return nil
}

// answerable tells whether a free answer can be written at the path (the rule
// of the answer endpoint, core.FieldAnswerable).
func (c *checker) answerable(p core.Path) bool {
	return core.FieldAnswerable(c.tc.Format, p)
}

// reviewOption controls an option of a review finding: its fills pass the checks
// of the fills of a task (value format, source proof, paths of the finding),
// only on fields of copied or judgment nature, and never over a value written or
// reviewed by the analyst. An option that writes nothing is refused.
func (c *checker) reviewOption(wo wireOption, paths []core.Path) (core.QuestionOption, string) {
	label := strings.TrimSpace(string(wo.Label))
	if label == "" {
		return core.QuestionOption{}, "empty label"
	}
	if len(wo.Fills) == 0 {
		return core.QuestionOption{}, "an option must carry fills (a finding without options is fine, an option that writes nothing is not)"
	}
	for _, w := range wo.Fills {
		path := core.Path(w.Path)
		if _, err := core.ParsePath(string(path)); err != nil {
			return core.QuestionOption{}, "invalid path: " + err.Error()
		}
		if !core.PathAllowed(path, paths) {
			return core.QuestionOption{}, fmt.Sprintf("%s is outside the paths of the finding", path)
		}
		node := c.tc.Format.Spec().NodeAt(path.Template())
		if node == nil || node.Kind != core.NodeField {
			return core.QuestionOption{}, fmt.Sprintf("%s is not a field", path)
		}
		if n := node.EffectiveNature(); n != core.NatureCopied && n != core.NatureJudgment {
			return core.QuestionOption{}, fmt.Sprintf("%s is a setting, computed or analyst field: it is never proposed", path)
		}
		if cur, ok := c.tc.Doc.Values[path]; ok && (cur.Origin == core.OriginManual || cur.Origin == core.OriginAnalyst || cur.Reviewed) {
			return core.QuestionOption{}, fmt.Sprintf("%s holds a value written or reviewed by the analyst: it is never replaced", path)
		}
	}
	// The same controls as the options of a question, within the finding's paths.
	cc := *c
	cc.allowed = paths
	fills, msg := cc.optionFills(wo.Fills, paths, c.tc.Doc)
	if msg != "" {
		return core.QuestionOption{}, msg
	}
	return core.QuestionOption{Label: label, Fills: fills}, ""
}

// checkStructured dry-runs a structured output on a copy of the document.
func (c *checker) checkStructured(final json.RawMessage) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.Newf("%v", r)
		}
	}()
	clone := c.tc.Doc.Clone()
	if err := c.tc.Format.ApplyStructured(c.spec.Kind, clone, final); err != nil {
		return err
	}
	if bad := sourcesOutsideSelection(c.tc.Doc, clone, c.sc); len(bad) > 0 {
		return errors.New(outsideSelectionMessage(bad))
	}
	return nil
}

// relationError checks that the value of a copied fill derives from the raw
// value of its cited source, by field type; msg is "" when it does (or when the
// relation is not checked). forceLow tells that the value is a correspondence
// the engine cannot prove (it needed a justification): its confidence is forced
// to low, whatever the model said. codes are the labels of the field's code
// list (nil when none).
//
//   - boolean: a boolean literal source must match; any other source needs a
//     justification and a confidence (forced low);
//   - code list: the value or its label matches the raw value (folded like text
//     proofs), else justification and confidence are required (forced low on an
//     exported field, a working node is left alone);
//   - decimal: the number of raw, or value x 100 == raw exactly (cents to euros)
//     with a justification (forced low).
func relationError(n *core.Node, codes []core.Code, value, raw, justification string, confidence core.Confidence) (msg string, forceLow bool) {
	mismatch := fmt.Sprintf("the value does not match the cited source value (source value: \"%s\")", shorten(raw, 100))
	judged := func(what string) (string, bool) {
		if strings.TrimSpace(justification) == "" || confidence == core.ConfidenceNone {
			return "justify the correspondence between the source value and " + what + " (justification and confidence level)", false
		}
		return "", n.Export
	}
	switch {
	case n.Type == core.TypeRef:
		return "", false
	case n.Type == core.TypeBoolean:
		rb, isLit := boolLiteral(raw)
		if !isLit {
			return judged("the chosen boolean")
		}
		if vb, ok := boolLiteral(value); !ok || vb != rb {
			return mismatch, false
		}
		return "", false
	case n.Type == core.TypeCode || n.CodeList != "":
		if foldAlnum(value) == foldAlnum(raw) || slices.ContainsFunc(codes, func(c core.Code) bool {
			return c.Value == value && c.Label != "" && foldAlnum(c.Label) == foldAlnum(raw)
		}) {
			return "", false
		}
		return judged("the chosen code")
	case n.Type == core.TypeDate:
		d, ok := calendarDay(raw)
		if !ok || d != value {
			return mismatch, false
		}
		return "", false
	case n.Type == core.TypeDecimal, n.Type == core.TypeInteger:
		if numberDerives(value, raw) {
			return "", false
		}
		if n.Type == core.TypeDecimal && centsOf(value, raw) {
			if strings.TrimSpace(justification) == "" {
				return "justify the conversion from cents to euros (justification)", false
			}
			return "", true
		}
		return mismatch, false
	}
	if n.IsItemRef(value) {
		return "", false
	}
	if !textDerives(value, raw, n.MaxLength) {
		return mismatch, false
	}
	return "", false
}

// centsOf tells whether value x 100 == raw, exactly (no float). A count of
// cents is an integer: a fractional raw value is refused.
func centsOf(value, raw string) bool {
	value, raw = stripThousands(value), stripThousands(raw)
	if !decimalRe.MatchString(value) || !integerRe.MatchString(raw) {
		return false
	}
	v, ok1 := new(big.Rat).SetString(strings.Replace(value, ",", ".", 1))
	r, ok2 := new(big.Rat).SetString(strings.Replace(raw, ",", ".", 1))
	if !ok1 || !ok2 {
		return false
	}
	return v.Mul(v, big.NewRat(100, 1)).Cmp(r) == 0
}

// boolLiteral reads a boolean written as true/false/1/0/oui/non/yes/no (any
// case); ok is false for any other text.
func boolLiteral(s string) (value, ok bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "1", "oui", "yes":
		return true, true
	case "false", "0", "non", "no":
		return false, true
	}
	return false, false
}

var numberTokenRe = regexp.MustCompile(`-?[0-9]+(?:[.,][0-9]+)?`)

// stripThousands removes the spaces (regular, no-break, narrow no-break) used
// as thousands separators.
func stripThousands(s string) string {
	return strings.NewReplacer(" ", "", "\u00a0", "", "\u202f", "", "\t", "").Replace(strings.TrimSpace(s))
}

// numberDerives: the value is the number of raw (thousands separators ignored,
// "1 000" is 1000), or one numeric token of a non-numeric raw ("12" from
// "12 rue du Test").
func numberDerives(value, raw string) bool {
	value = stripThousands(value)
	if !decimalRe.MatchString(value) {
		return false
	}
	nv := normDecimal(strings.TrimPrefix(value, "-"))
	if whole := stripThousands(raw); decimalRe.MatchString(whole) {
		return normDecimal(strings.TrimPrefix(whole, "-")) == nv
	}
	for _, tok := range numberTokenRe.FindAllString(raw, -1) {
		if normDecimal(strings.TrimPrefix(tok, "-")) == nv {
			return true
		}
	}
	return false
}

func shorten(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// foldAlnum folds case and accents and drops everything but letters and digits.
func foldAlnum(s string) string {
	return strings.Join(foldTokens(s), "")
}

// foldTokens folds s and splits it on anything but letters and digits.
func foldTokens(s string) []string {
	return strings.FieldsFunc(foldText(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// textDerives: folded equality, a contiguous token subsequence of the raw text
// (name or address split), or a truncation to the maximum length.
func textDerives(value, raw string, maxLen int) bool {
	vt, rt := foldTokens(value), foldTokens(raw)
	if len(vt) == 0 {
		return false
	}
	vj, rj := strings.Join(vt, ""), strings.Join(rt, "")
	if vj == rj {
		return true
	}
	for i := 0; i+len(vt) <= len(rt); i++ {
		if slices.Equal(rt[i:i+len(vt)], vt) {
			return true
		}
	}
	return maxLen > 0 && len([]rune(value)) >= maxLen && strings.HasPrefix(rj, vj)
}

// calendarDay returns the AAAA-MM-JJ day of a raw date or timestamp (Paris day
// for a timestamp).
func calendarDay(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if day, ok := parisDay(raw); ok {
		return day, true
	}
	for _, layout := range []string{"2006-01-02", "02/01/2006"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.Format("2006-01-02"), true
		}
	}
	return "", false
}

// dryRun is the validate tool of a task: the candidate fills go through the same
// control as the fills of a final answer (paths of the task's part, value
// format, source and proof), nothing is written. It reports what would be
// refused, what would be turned into a question and how many fills would pass.
func (c *checker) dryRun(args json.RawMessage) string {
	var a validateArgs
	if msg, ok := parseArgs(args, &a); !ok {
		return msg
	}
	if len(a.Fills) > validateMaxFills {
		return toolError(fmt.Sprintf("too many values to validate (%d at most)", validateMaxFills))
	}
	raw, err := json.Marshal(wireOutput{Fills: a.Fills})
	if err != nil {
		return toolError("unreadable fills")
	}
	res := c.checkFills(raw)
	questions := make([]string, 0, len(res.Questions))
	for _, q := range res.Questions {
		questions = append(questions, q.Text)
	}
	return toolJSON(map[string]any{
		"accepted":  len(res.Fills),
		"problems":  append([]string{}, res.Corrections...),
		"notes":     append([]string{}, res.Notes...),
		"would_ask": questions,
	})
}
