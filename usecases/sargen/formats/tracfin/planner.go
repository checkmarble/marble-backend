package tracfin

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// Step kinds of the TRACFIN plan. The other kinds are core.TaskKindCompute and
// core.TaskKindReview.
const (
	StepPlacement       = "placement"
	StepFillPerson      = "fill_person"
	StepFillTransaction = "fill_transaction"
	StepJudgment        = "judgment"
	StepCorrection      = "correction"
)

const (
	transactionBatchSize = 10
	maxCorrectionWaves   = 2
)

// PayloadDeclared is the payload block of the person and transaction tasks listing the
// declared persons and financial references (path, kind, owner, sources).
const PayloadDeclared = "declared_persons_and_references"

// Tool names (the common tools are those of the engine).
const (
	toolOperationPreview = "tracfin_operation_preview"
	toolReadDocument     = "read_document"
)

var (
	commonReadTools = []string{"read_object", "follow_link", "field_spec", "search_code", "data_model"}
	fillTools       = append(slices.Clone(commonReadTools), "validate")
)

// judgmentFields are the person fields only the judgment step writes.
var judgmentFields = []string{"implication", "natureRelation", "elementsRelation"}

var personKinds = []string{"pp", "pm", "pnc"}

// formCodes are the blocking issue codes a correction task may fix: the value
// is there but its form is wrong. A missing value (required, too_few_items) is
// a question, never a correction.
var formCodes = []string{
	core.CodeBadDate, core.CodeBadBoolean, core.CodeBadInteger, core.CodeBadDecimal, core.CodeBadRef,
	core.CodeDanglingRef, core.CodeMaxLength, core.CodeMinLength, core.CodePattern, core.CodeUnknownCode,
	core.CodeBadChar, core.CodeBadPattern,
}

// Plan returns the tasks to run next, one wave per call: placement, then the
// person and transaction fills with the computation that follows them, then
// the judgment, then at most two waves of corrections, then the review (once).
// A review job only plans the review. A computed operation line whose group
// attributes change gets a new path: questions on the old line path go stale. The specs depend only on the state (no
// clock, no random value), so a resumed job finds its tasks again.
func (f *format) Plan(st core.PlanState) []core.TaskSpec {
	done := newDoneSet(st.Done)
	if st.Doc == nil {
		st.Doc = core.NewDocument()
	}

	if st.JobKind == string(models.SarJobKindReview) {
		if done.kind(core.TaskKindReview) {
			return nil
		}
		return []core.TaskSpec{f.reviewSpec(st)}
	}

	if !done.kind(StepPlacement) {
		return []core.TaskSpec{f.placementSpec(st)}
	}

	// Fills, then the computation that must follow them. A placement that
	// placed nothing has no fill: the plan goes on and the review and the
	// to-do list show what is missing.
	var wave []core.TaskSpec
	for _, s := range f.fillSpecs(st) {
		if !done.spec(s) {
			wave = append(wave, s)
		}
	}
	if !done.kind(core.TaskKindCompute) {
		wave = append(wave, computeSpec())
	}
	if len(wave) > 0 {
		return wave
	}

	if !done.kind(StepJudgment) {
		return []core.TaskSpec{f.judgmentSpec(st)}
	}

	n := done.correctionWaves()
	if n > 0 && done.computeWave < n {
		// A correction wave is done: recompute before the next wave and the
		// review, which must not see stale computed values.
		return []core.TaskSpec{computeAfterCorrectionSpec(n)}
	}
	if n < maxCorrectionWaves {
		if corrections := f.correctionSpecs(st, n+1); len(corrections) > 0 {
			return corrections
		}
	}

	if !done.kind(core.TaskKindReview) {
		return []core.TaskSpec{f.reviewSpec(st)}
	}
	return nil
}

// doneSet answers "is this task already done" from the done tasks of the job.
type doneSet struct {
	kinds map[string]bool
	specs map[string]bool
	waves int
	// computeWave is the highest correction wave a done compute task followed.
	computeWave int
}

func newDoneSet(tasks []models.SarGenerationTask) doneSet {
	d := doneSet{kinds: map[string]bool{}, specs: map[string]bool{}}
	for _, t := range tasks {
		d.kinds[t.Kind] = true
		var in struct {
			Spec core.TaskSpec `json:"spec"`
		}
		spec := core.TaskSpec{Kind: t.Kind, TargetPath: core.Path(t.TargetPath)}
		if json.Unmarshal(t.Input, &in) == nil && in.Spec.Kind == t.Kind {
			spec = in.Spec
		}
		d.specs[specKey(spec)] = true
		switch t.Kind {
		case StepCorrection:
			d.waves = max(d.waves, waveOf(spec))
		case core.TaskKindCompute:
			d.computeWave = max(d.computeWave, waveOf(spec))
		}
	}
	return d
}

func (d doneSet) kind(k string) bool        { return d.kinds[k] }
func (d doneSet) spec(s core.TaskSpec) bool { return d.specs[specKey(s)] }
func (d doneSet) correctionWaves() int      { return d.waves }
func waveOf(s core.TaskSpec) int {
	switch w := s.Payload["wave"].(type) {
	case int:
		return w
	case float64:
		return int(w)
	}
	return 0
}

// specKey identifies a task across plans: its kind, its target, its allowed
// paths and, for corrections, its wave.
func specKey(s core.TaskSpec) string {
	allowed := make([]string, len(s.AllowedPaths))
	for i, p := range s.AllowedPaths {
		allowed[i] = string(p)
	}
	wave := ""
	if s.Kind == StepCorrection {
		wave = fmt.Sprint(waveOf(s))
	}
	return s.Kind + "\x00" + string(s.TargetPath) + "\x00" + strings.Join(allowed, "\x01") + "\x00" + wave
}

func selectionRefs(sel []models.SarSelectedObject) []core.ObjectRef {
	out := make([]core.ObjectRef, 0, len(sel))
	for _, o := range sel {
		out = append(out, core.ObjectRef{Table: o.TableName, ObjectID: o.ObjectId})
	}
	return out
}

func (f *format) placementSpec(st core.PlanState) core.TaskSpec {
	schema := placementSchema(f.referenceKinds())
	return core.TaskSpec{
		Kind: StepPlacement, TargetPath: "personnes",
		AllowedPaths: []core.Path{"personnes", transactionsContainer},
		Inputs:       selectionRefs(st.Selection),
		PromptPath:   models.SarGenerationTracfinPlacementPromptPath,
		Tier:         core.TierLight,
		ToolNames:    slices.Clone(commonReadTools),
		OutputSchema: &schema,
		// The whole generation depends on the placement: a placement cut short by
		// a limit must not leave an empty declaration.
		FailJobOnLimit: true,
		Limits:         placementLimits(len(st.Selection)),
	}
}

// Placement budget: it reads and places every selected object, so it grows
// with the selection n: tool calls 30 + n/2 (cap 300), turns 15 + n/4 (cap 150),
// time 3 min + 3 s per object (cap 20 min). At n = 10 this is 35 calls, 17 turns,
// 3.5 min; at n = 200, 130 calls, 65 turns, 13 min.
func placementLimits(n int) core.TaskLimits {
	return core.TaskLimits{
		MaxToolCalls:   min(30+n/2, 300),
		MaxTurns:       min(15+n/4, 150),
		TimeoutSeconds: min(180+3*n, 1200),
	}
}

// personSpecs returns the fill tasks, one per person item.
func (f *format) fillSpecs(st core.PlanState) []core.TaskSpec {
	var out []core.TaskSpec
	for _, kind := range personKinds {
		container := core.Path("personnes." + kind)
		for _, key := range st.Doc.Items[container] {
			item := core.Path(string(container) + "[" + key + "]")
			out = append(out, core.TaskSpec{
				Kind: StepFillPerson, TargetPath: item,
				AllowedPaths: f.personFillPaths(kind, item, st.Doc),
				Inputs:       slices.Clone(st.Doc.ItemSources[item]),
				PromptPath:   models.SarGenerationTracfinFillPersonPromptPath,
				Tier:         core.TierLight,
				ToolNames:    slices.Clone(fillTools),
				Payload:      map[string]any{PayloadDeclared: f.declaredBlock(st.Doc)},
			})
		}
	}
	keys := st.Doc.Items[transactionsContainer]
	for start := 0; start < len(keys); start += transactionBatchSize {
		batch := keys[start:min(start+transactionBatchSize, len(keys))]
		spec := core.TaskSpec{
			Kind: StepFillTransaction, TargetPath: transactionsContainer,
			PromptPath: models.SarGenerationTracfinFillTransactionPromptPath,
			Tier:       core.TierLight,
			ToolNames:  append(slices.Clone(fillTools), toolOperationPreview, toolReadDocument),
			Payload:    map[string]any{PayloadDeclared: f.declaredBlock(st.Doc)},
		}
		for _, k := range batch {
			item := core.Path(string(transactionsContainer) + "[" + k + "]")
			spec.AllowedPaths = append(spec.AllowedPaths, item)
			spec.Inputs = append(spec.Inputs, st.Doc.ItemSources[item]...)
		}
		spec.Inputs = dedupeRefs(spec.Inputs)
		out = append(out, spec)
	}
	return out
}

// personFillPaths lists what a person fill task may write: everything of the
// person but the relation fields (the judgment's), its export id and its
// financial references; of those, only the fields of the references the
// placement declared (never their lien, never a new reference).
func (f *format) personFillPaths(kind string, item core.Path, doc *core.Document) []core.Path {
	node := f.spec.NodeAt("personnes." + kind + "[]")
	if node == nil {
		return []core.Path{item}
	}
	var out []core.Path
	for _, c := range node.Children {
		if c.ExportID || slices.Contains(judgmentFields, c.Name) {
			continue
		}
		if c.Name == "referencesFinancieres" {
			for _, rk := range c.Children {
				container := item.Child(c.Name).Child(rk.Name)
				for _, key := range doc.Items[container] {
					ref := core.Path(string(container) + "[" + key + "]")
					for _, field := range rk.Children {
						if field.ExportID || field.Name == "lien" {
							continue
						}
						out = append(out, ref.Child(field.Name))
					}
				}
			}
			continue
		}
		out = append(out, item.Child(c.Name))
	}
	return out
}

// declaredBlock lists the persons and financial references the placement
// declared (path, kind, owner, source objects), in document order: what a
// transaction task needs to name them. It holds no value, so it never changes
// while the fills run (the specs stay deterministic).
func (f *format) declaredBlock(doc *core.Document) []map[string]any {
	refs := func(p core.Path) []map[string]string {
		out := []map[string]string{}
		for _, o := range doc.ItemSources[p] {
			out = append(out, map[string]string{"table": o.Table, "object_id": o.ObjectID})
		}
		return out
	}
	var out []map[string]any
	for _, kind := range personKinds {
		container := core.Path("personnes." + kind)
		for _, key := range doc.Items[container] {
			person := core.Path(string(container) + "[" + key + "]")
			out = append(out, map[string]any{"path": string(person), "kind": kind, "sources": refs(person)})
			node := f.spec.NodeAt("personnes." + kind + "[].referencesFinancieres")
			if node == nil {
				continue
			}
			for _, rk := range node.Children {
				rc := person.Child("referencesFinancieres").Child(rk.Name)
				for _, rkey := range doc.Items[rc] {
					ref := core.Path(string(rc) + "[" + rkey + "]")
					out = append(out, map[string]any{
						"path": string(ref), "kind": strings.TrimPrefix(rk.Name, referenceContainerPrefix),
						"owner": string(person), "sources": refs(ref),
					})
				}
			}
		}
	}
	return out
}

func computeSpec() core.TaskSpec {
	return core.TaskSpec{
		Kind: core.TaskKindCompute, TargetPath: "operations",
		DependsOnKinds: []string{StepFillPerson, StepFillTransaction},
	}
}

// computeAfterCorrectionSpec recomputes after a correction wave (the wave in the
// payload makes it a distinct task).
func computeAfterCorrectionSpec(wave int) core.TaskSpec {
	return core.TaskSpec{
		Kind: core.TaskKindCompute, TargetPath: "operations", DependsOnKinds: []string{StepCorrection},
		Payload: map[string]any{"wave": wave},
	}
}

func (f *format) judgmentSpec(st core.PlanState) core.TaskSpec {
	allowed := []core.Path{"context.natureSoupcon", "context.information"}
	for _, kind := range personKinds {
		container := core.Path("personnes." + kind)
		for _, key := range st.Doc.Items[container] {
			item := core.Path(string(container) + "[" + key + "]")
			for _, name := range judgmentFields {
				if f.spec.NodeAt("personnes."+kind+"[]."+name) != nil {
					allowed = append(allowed, item.Child(name))
				}
			}
		}
	}
	return core.TaskSpec{
		Kind: StepJudgment, TargetPath: "context", AllowedPaths: allowed,
		Inputs:     selectionRefs(st.Selection),
		PromptPath: models.SarGenerationTracfinJudgmentPromptPath,
		Tier:       core.TierHeavy,
		ToolNames:  append(slices.Clone(fillTools), toolReadDocument),
		Payload:    analystPayload(st),

		NeedsCaseContext: true,
	}
}

func (f *format) reviewSpec(st core.PlanState) core.TaskSpec {
	return core.TaskSpec{
		Kind:       core.TaskKindReview,
		Inputs:     selectionRefs(st.Selection),
		PromptPath: models.SarGenerationTracfinReviewPromptPath,
		Tier:       core.TierHeavy,
		ToolNames:  append(slices.Clone(commonReadTools), toolReadDocument),
		Payload:    analystPayload(st),

		NeedsCaseContext: true,
	}
}

// analystPayload is what the analyst wrote: the analysis and the additional
// information. Absent when empty.
func analystPayload(st core.PlanState) map[string]any {
	p := map[string]any{}
	if a := strings.TrimSpace(st.Declaration.Analysis); a != "" {
		p["analysis"] = a
	}
	if e := strings.TrimSpace(st.Declaration.ExtraInfo); e != "" {
		p["extra_info"] = e
	}
	if len(p) == 0 {
		return nil
	}
	return p
}

// correctionSpecs plans one correction task per root item holding blocking
// form issues, for the given wave. Computed, setting and analyst fields are
// out of reach of the model and missing values are questions: neither is
// corrected.
func (f *format) correctionSpecs(st core.PlanState, wave int) []core.TaskSpec {
	byRoot := map[core.Path][]core.Issue{}
	var roots []core.Path
	for _, is := range st.Issues {
		if is.Severity != core.SeverityBlocking || !slices.Contains(formCodes, is.Code) || !f.correctable(is.Path) {
			continue
		}
		root := rootOf(is.Path)
		if _, seen := byRoot[root]; !seen {
			roots = append(roots, root)
		}
		byRoot[root] = append(byRoot[root], is)
	}
	slices.Sort(roots)
	var out []core.TaskSpec
	for _, root := range roots {
		issues := byRoot[root]
		slices.SortFunc(issues, func(a, b core.Issue) int { return strings.Compare(string(a.Path)+a.Code, string(b.Path)+b.Code) })
		var allowed []core.Path
		var listed []map[string]string
		for _, is := range issues {
			if !slices.Contains(allowed, is.Path) {
				allowed = append(allowed, is.Path)
			}
			listed = append(listed, map[string]string{"path": string(is.Path), "code": is.Code, "message": is.Message})
		}
		out = append(out, core.TaskSpec{
			Kind: StepCorrection, TargetPath: root, AllowedPaths: allowed,
			Inputs:     slices.Clone(st.Doc.ItemSources[root]),
			PromptPath: models.SarGenerationTracfinCorrectionPromptPath,
			Tier:       core.TierLight,
			ToolNames:  append(slices.Clone(fillTools), toolReadDocument),
			Payload:    map[string]any{"wave": wave, "issues": listed},
		})
	}
	return out
}

// correctable tells whether the model may write the field of an issue.
func (f *format) correctable(p core.Path) bool {
	n := f.spec.NodeAt(p.Template())
	if n == nil || n.Kind != core.NodeField {
		return false
	}
	switch n.EffectiveNature() {
	case core.NatureCopied, core.NatureJudgment:
		return true
	}
	return false
}

// rootOf is the item holding a path (up to its first keyed segment), else the
// top-level section.
func rootOf(p core.Path) core.Path {
	segs := strings.Split(string(p), ".")
	for i, s := range segs {
		if strings.HasSuffix(s, "]") {
			return core.Path(strings.Join(segs[:i+1], "."))
		}
	}
	return core.Path(segs[0])
}

func dedupeRefs(refs []core.ObjectRef) []core.ObjectRef {
	var out []core.ObjectRef
	for _, r := range refs {
		if !slices.Contains(out, r) {
			out = append(out, r)
		}
	}
	return out
}
