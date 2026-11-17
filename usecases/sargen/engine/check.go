package engine

import (
	"encoding/json"
	"fmt"
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
}

// checker applies the output control to the final answers of one task run.
type checker struct {
	tc      core.ToolContext
	spec    core.TaskSpec
	taskID  uuid.UUID
	allowed []core.Path
	sc      scope
}

func newChecker(tc core.ToolContext, spec core.TaskSpec, taskID uuid.UUID, allowed []core.Path) *checker {
	return &checker{tc: tc, spec: spec, taskID: taskID, allowed: allowed, sc: newScope(tc.Selection)}
}

// checkResult is the outcome of the control of one final answer.
type checkResult struct {
	Fills     []core.Fill
	Questions []core.QuestionDraft
	// Corrections are the form errors to send back to the LLM.
	Corrections []string
	// Notes are control remarks kept in the trace only.
	Notes []string
}

// neverSendBack lists the issue codes that are completeness matters of the
// whole document (checked by the validator, shown to the analyst), never errors
// the LLM could fix by rewriting its answer.
func neverSendBack(code string) bool {
	return code == core.CodeRequired || code == core.CodeTooFewItems
}

// checkFills controls a final answer of a fill task.
func (c *checker) checkFills(final json.RawMessage) checkResult {
	var res checkResult
	var out wireOutput
	if err := json.Unmarshal(final, &out); err != nil {
		res.Corrections = append(res.Corrections, "la réponse n'est pas un JSON conforme au schéma demandé : "+err.Error())
		return res
	}

	type pending struct {
		idx  int
		fill core.Fill
	}
	var good []pending
	for i, w := range out.Fills {
		path := core.Path(w.Path)
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
			res.Notes = append(res.Notes, fmt.Sprintf("fills[%d] %s : valeur vide ignorée", i, path))
			continue
		}
		f, msg := c.toFill(w)
		if msg != "" {
			res.Corrections = append(res.Corrections, fmt.Sprintf("fills[%d] %s : %s", i, path, msg))
			continue
		}
		good = append(good, pending{i, f})
	}

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
	for i, g := range good {
		issue, bad := rejectedIdx[i]
		if !bad {
			res.Fills = append(res.Fills, g.fill)
			continue
		}
		if neverSendBack(issue.Code) {
			res.Questions = append(res.Questions, unverifiable(g.fill, issue.Message))
			continue
		}
		res.Corrections = append(res.Corrections, fmt.Sprintf("fills[%d] %s : %s", g.idx, g.fill.Path, issue.Message))
	}

	c.checkQuestions(&res, out.Questions, clone)
	return res
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
				res.Notes = append(res.Notes, fmt.Sprintf("questions[%d] : chemin hors de la partie confiée ignoré : %s", qi, p))
			}
		}
		for oi, wo := range wq.Options {
			opt := core.QuestionOption{Label: string(wo.Label)}
			fills, note := c.optionFills(wo.Fills, q.Paths, afterFills)
			if note != "" {
				res.Notes = append(res.Notes, fmt.Sprintf("questions[%d].options[%d] : %s ; gardée comme simple libellé", qi, oi, note))
			}
			opt.Fills = fills
			q.Options = append(q.Options, opt)
		}
		if len(wq.Paths) > 0 && len(q.Paths) == 0 {
			res.Notes = append(res.Notes, fmt.Sprintf("questions[%d] : plus aucun chemin dans la partie confiée, question ignorée", qi))
			continue
		}
		res.Questions = append(res.Questions, q)
	}
}

// optionFills returns the fills of an option when all pass, else none and the
// reason.
func (c *checker) optionFills(ws []wireFill, questionPaths []core.Path, doc *core.Document) ([]core.Fill, string) {
	if len(ws) == 0 {
		return nil, ""
	}
	fills := make([]core.Fill, 0, len(ws))
	for _, w := range ws {
		if w.Value == "" {
			return nil, "valeur vide"
		}
		f, msg := c.toFill(w)
		if msg != "" {
			return nil, msg
		}
		if !core.PathAllowed(f.Path, questionPaths) {
			return nil, fmt.Sprintf("%s hors des champs de la question", f.Path)
		}
		fills = append(fills, f)
	}
	clone := doc.Clone()
	res := core.ApplyFills(c.tc.Format.Spec(), clone, fills, c.applyOptions(clone))
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
func (c *checker) toFill(w wireFill) (core.Fill, string) {
	path := core.Path(w.Path)
	if _, err := core.ParsePath(string(path)); err != nil {
		return core.Fill{}, "chemin invalide : " + err.Error()
	}
	if !c.pathAllowed(path) {
		return core.Fill{}, "chemin hors de la partie confiée à cette tâche ; chemins autorisés : " + joinPaths(c.allowed)
	}
	f := core.Fill{
		Path: path, Value: string(w.Value), Origin: core.OriginLLM, TaskID: &c.taskID,
	}
	if w.RuleID != nil && *w.RuleID != "" {
		f.RuleIDs = []string{string(*w.RuleID)}
	}
	if w.Justification != nil {
		f.Justification = string(*w.Justification)
	}
	if w.Confidence != nil {
		f.Confidence = core.Confidence(*w.Confidence)
	}
	if w.Source != nil {
		src, msg := c.toSource(path, *w.Source)
		if msg != "" {
			return core.Fill{}, msg
		}
		f.Sources = []core.SourceRef{src}
		if node := c.tc.Format.Spec().NodeAt(path.Template()); node != nil && node.Kind == core.NodeField &&
			node.EffectiveNature() == core.NatureCopied {
			if msg := relationError(node, f.Value, src.RawValue, f.Justification); msg != "" {
				return core.Fill{}, msg
			}
		}
	}
	return f, ""
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
		return core.SourceRef{}, "source sans champ (field)"
	}
	ref := core.ObjectRef{Table: string(w.Table), ObjectID: string(w.ObjectID)}
	switch {
	case ref.Table == "" && ref.ObjectID == "":
		var msg string
		if ref, msg = c.defaultSource(target); msg != "" {
			return core.SourceRef{}, msg
		}
	case ref.Table == "" || ref.ObjectID == "":
		return core.SourceRef{}, "table et object_id vont ensemble : donne les deux ou aucun"
	}
	obj, ok := c.sc.get(ref.Table, ref.ObjectID)
	if !ok {
		return core.SourceRef{}, "l'objet source " + ref.Table + "/" + ref.ObjectID + " ne fait pas partie de la sélection"
	}
	if !proves(obj, string(w.Field), string(w.RawValue)) {
		return core.SourceRef{}, fmt.Sprintf(
			"raw_value ne correspond pas à la valeur du champ %q de l'objet %s/%s : relis l'objet avec read_object et recopie la valeur exacte",
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
			return core.ObjectRef{}, "l'élément a plusieurs objets sources (" + strings.Join(names, ", ") + ") : précise table et object_id"
		}
	}
	if len(c.spec.Inputs) == 1 {
		return c.spec.Inputs[0], ""
	}
	return core.ObjectRef{}, "cet élément n'a pas d'objet source par défaut : précise table et object_id"
}

var (
	decimalRe = regexp.MustCompile(`^-?[0-9]+([.,][0-9]+)?$`)
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
		return nil, []string{"la réponse n'est pas un JSON conforme au schéma demandé : " + err.Error()}
	}
	var (
		findings []core.ReviewFinding
		errs     []string
	)
	for i, w := range out.Findings {
		f := core.ReviewFinding{Severity: string(w.Severity), Message: strings.TrimSpace(string(w.Message))}
		bad := false
		if f.Severity != "important" && f.Severity != "mineur" {
			errs = append(errs, fmt.Sprintf("findings[%d] : severity doit valoir important ou mineur", i))
			bad = true
		}
		if f.Message == "" {
			errs = append(errs, fmt.Sprintf("findings[%d] : message vide", i))
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
		if !bad {
			findings = append(findings, f)
		}
	}
	return findings, errs
}

// checkStructured dry-runs a structured output on a copy of the document.
func (c *checker) checkStructured(final json.RawMessage) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.Newf("%v", r)
		}
	}()
	return c.tc.Format.ApplyStructured(c.spec.Kind, c.tc.Doc.Clone(), final)
}

// relationError checks that the value of a copied fill derives from the raw
// value of its cited source, by field type; "" when it does (or when the
// relation is not checked).
func relationError(n *core.Node, value, raw, justification string) string {
	mismatch := fmt.Sprintf("la valeur ne correspond pas à la valeur source citée (valeur source : « %s »)", shorten(raw, 100))
	switch {
	case n.Type == core.TypeRef:
		return ""
	case n.Type == core.TypeBoolean:
		rb, isLit := boolLiteral(raw)
		if !isLit {
			if strings.TrimSpace(justification) == "" {
				return "justifiez la correspondance entre la valeur source et le booléen choisi (justification)"
			}
			return ""
		}
		if vb, ok := boolLiteral(value); !ok || vb != rb {
			return mismatch
		}
		return ""
	case n.Type == core.TypeCode || n.CodeList != "":
		if foldAlnum(value) != foldAlnum(raw) && strings.TrimSpace(justification) == "" {
			return "justifiez la correspondance entre la valeur source et le code choisi (justification)"
		}
		return ""
	case n.Type == core.TypeDate:
		d, ok := calendarDay(raw)
		if !ok || d != value {
			return mismatch
		}
		return ""
	case n.Type == core.TypeDecimal, n.Type == core.TypeInteger:
		if !numberDerives(value, raw) {
			return mismatch
		}
		return ""
	}
	if n.IsItemRef(value) {
		return ""
	}
	if !textDerives(value, raw, n.MaxLength) {
		return mismatch
	}
	return ""
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
