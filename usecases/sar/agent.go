package sar

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/invopop/jsonschema"
)

//go:embed prompts/*.md
var promptFiles embed.FS

// LLM is the model adapter. Tests use a scripted implementation.
type LLM interface {
	Complete(ctx context.Context, system, prompt string, schema *jsonschema.Schema) (string, error)
}

var ErrBudget = errors.New("SAR agent call budget exhausted")

// AIOp is a patch operation proposed by the model. Values are split by
// shape so that the response schema stays simple for structured output.
type AIOp struct {
	Op          string   `json:"op" jsonschema:"enum=set,enum=add,enum=remove"`
	Path        string   `json:"path"`
	Value       string   `json:"value,omitempty"`
	Values      []string `json:"values,omitempty"`
	Object      string   `json:"object,omitempty" jsonschema:"description=JSON object encoded as a string, for add of a new pivot object"`
	Sources     []string `json:"sources"`
	Explanation string   `json:"explanation"`
}

type AIPivotOutput struct {
	Ops     []AIOp   `json:"ops"`
	Missing []string `json:"missing,omitempty"`
}

type AIReviewFinding struct {
	Step    string `json:"step"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

type AIReviewOutput struct {
	Findings []AIReviewFinding `json:"findings"`
}

type AIExtValue struct {
	Key         string   `json:"key"`
	ObjectID    string   `json:"object_id,omitempty"`
	Values      []string `json:"values"`
	Sources     []string `json:"sources"`
	Explanation string   `json:"explanation"`
}

type AIExtOutput struct {
	Values []AIExtValue `json:"values"`
}

type AIChatOutput struct {
	Reply   string `json:"reply"`
	Refused bool   `json:"refused"`
	Ops     []AIOp `json:"ops"`
}

func (o AIOp) Patch() (models.SarPatchOp, error) {
	p := models.SarPatchOp{Op: o.Op, Path: o.Path}
	switch {
	case o.Object != "":
		var v map[string]any
		if err := json.Unmarshal([]byte(o.Object), &v); err != nil {
			return p, fmt.Errorf("object is not a JSON object: %w", err)
		}
		p.Value, _ = json.Marshal(v)
	case o.Values != nil:
		p.Value, _ = json.Marshal(o.Values)
	case o.Op != "remove":
		p.Value, _ = json.Marshal(o.Value)
	}
	return p, nil
}

// Step describes one agent step on the pivot.
type Step struct {
	Name         string
	Prefixes     []string
	Instructions string
}

var PivotSteps = []Step{
	{Name: "parties", Prefixes: []string{"pivot/parties"}, Instructions: "Identifie toutes les parties impliquées : clients, suspects, victimes, contreparties, dirigeants, bénéficiaires effectifs, signataires. Pour chacune : type (person, entity, unknown), rôles neutres, statut client (is_client) et nature de la relation, identité, adresses, pièces d'identité lues dans les documents. Complète les parties déjà créées par le mapping au lieu d'en créer des doublons. Si une adresse n'est qu'un texte unique (ex. « 12 rue X 75001 Paris » dans street), décompose-la en number, street, postal_code et city en citant la même source."},
	{Name: "instruments", Prefixes: []string{"pivot/instruments", "pivot/links"}, Instructions: "Identifie les supports financiers (comptes/IBAN, cartes, portefeuilles) et les liens : titulaire, co-titulaire, mandataire/signataire entre une partie et un support ; dirigeant, bénéficiaire effectif, actionnaire entre deux parties (party_id vers to_party_id). Indique held_at_reporter=true pour les supports tenus par l'établissement déclarant."},
	{Name: "operations", Prefixes: []string{"pivot/operations"}, Instructions: "Choisis les opérations pertinentes pour le soupçon : selected=false pour les opérations sans rapport. Pour chaque opération retenue : sens (outgoing/incoming du point de vue du client), type neutre, canal, statut (executed, attempted, refused, pending), client (party_id) et son support, contrepartie et son support, suspicious. Ne change pas les montants, devises et horodatages issus du data model."},
	{Name: "attachments", Prefixes: []string{"pivot/attachments", "pivot/parties"}, Instructions: "Sélectionne les pièces qui fondent la déclaration (selected=true), avec leur type documentaire, leur date et une description courte. Rattache une pièce d'identité à la partie concernée (parties/<id>/identifications avec attachment_id) et reporte les données lues (numéro, pays, dates)."},
	{Name: "suspicion", Prefixes: []string{"pivot/suspicion", "pivot/prior_reports"}, Instructions: "Renseigne les thèmes neutres du soupçon (liste theme), un résumé factuel court, les mesures prises par l'établissement, la date de détection, urgent et judicial_origin si les sources l'indiquent, et les déclarations antérieures citées. N'écris jamais l'analyse (suspicion/analysis)."},
}

// Agent drives the bounded loop: input -> model output -> backend
// validation -> diagnostics -> new output, within a call budget.
type Agent struct {
	LLM     LLM
	Budget  int
	Used    int
	RunID   string
	Now     func() time.Time
	Sources []models.TracfinAISource
	Trace   func(kind, step string, attempt int, data any)
	// Attempts per step: one generation plus repairs.
	MaxAttempts int
}

func (ag *Agent) trace(kind, step string, attempt int, data any) {
	if ag.Trace != nil {
		ag.Trace(kind, step, attempt, data)
	}
}

func (ag *Agent) attempts() int {
	if ag.MaxAttempts <= 0 {
		return 3
	}
	return ag.MaxAttempts
}

func (ag *Agent) now() time.Time {
	if ag.Now != nil {
		return ag.Now()
	}
	return time.Now().UTC()
}

func render(name string, data any) (string, error) {
	raw, err := promptFiles.ReadFile("prompts/" + name)
	if err != nil {
		return "", err
	}
	t, err := template.New(name).Option("missingkey=error").Parse(string(raw))
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	err = t.Execute(&b, data)
	return b.String(), err
}

func (ag *Agent) call(ctx context.Context, step string, attempt int, prompt string, out any) (string, error) {
	if ag.Used >= ag.Budget {
		return "", ErrBudget
	}
	ag.Used++
	system, err := render("system.md", nil)
	if err != nil {
		return "", err
	}
	r := &jsonschema.Reflector{DoNotReference: true, ExpandedStruct: true}
	schema := r.Reflect(out)
	ag.trace("request", step, attempt, map[string]any{"prompt": prompt})
	text, err := ag.LLM.Complete(ctx, system, prompt, schema)
	ag.trace("response", step, attempt, map[string]any{"raw": text, "error": errString(err)})
	if err != nil {
		return "", err
	}
	return text, decode(text, out)
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func decode(text string, out any) error {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(strings.TrimSuffix(strings.TrimPrefix(text, "```json"), "```"), "```")
	dec := json.NewDecoder(strings.NewReader(text))
	dec.DisallowUnknownFields()
	return dec.Decode(out)
}

func (ag *Agent) sourceIDs() map[string]bool {
	ids := map[string]bool{"analysis": true}
	for _, s := range ag.Sources {
		ids[s.ID] = true
	}
	return ids
}

// SourcesText renders the context for prompts, bounded in size.
func SourcesText(sources []models.TracfinAISource, limit int) string {
	var b strings.Builder
	for _, s := range sources {
		if s.Kind == "object_path" || s.Kind == "configuration" {
			continue
		}
		content := string(s.Content)
		if len(content) > 20000 {
			content = content[:20000] + "…(tronqué)"
		}
		chunk := fmt.Sprintf("### %s (%s)\n%s\n\n", s.ID, s.Kind, content)
		if b.Len()+len(chunk) > limit {
			b.WriteString("…(contexte tronqué : limite de taille atteinte)\n")
			break
		}
		b.WriteString(chunk)
	}
	return b.String()
}

// CompactPivot renders the pivot for prompts: values only, manual marked.
func CompactPivot(p *models.SarPivot) string {
	m, _ := toMap(p)
	for _, k := range []string{"org_id", "revision", "updated_at", "updated_by", "validated"} {
		delete(m, k)
	}
	var simplify func(v any) any
	simplify = func(v any) any {
		switch x := v.(type) {
		case map[string]any:
			if val, ok := x["v"]; ok {
				if p, ok := x["p"].(map[string]any); ok && p["kind"] == models.SarProvManual {
					return map[string]any{"v": val, "manual": true}
				}
				return val
			}
			out := map[string]any{}
			for k, e := range x {
				if e == nil {
					continue
				}
				out[k] = simplify(e)
			}
			return out
		case []any:
			out := make([]any, 0, len(x))
			for _, e := range x {
				out = append(out, simplify(e))
			}
			return out
		}
		return v
	}
	raw, _ := json.MarshalIndent(simplify(m), "", " ")
	return string(raw)
}

func fieldsText(prefixes []string) string {
	var b strings.Builder
	vocabs := map[string]bool{}
	for _, f := range PivotFields {
		ok := false
		for _, p := range prefixes {
			if strings.HasPrefix("pivot/"+f.Path, p) {
				ok = true
			}
		}
		if !ok {
			continue
		}
		fmt.Fprintf(&b, "- `%s` (%s%s) : %s", f.Path, f.Kind, map[bool]string{true: ", liste"}[f.List], f.Label)
		if f.Vocab != "" {
			fmt.Fprintf(&b, " — codes `%s`", f.Vocab)
			vocabs[f.Vocab] = true
		}
		if f.Ref != "" {
			fmt.Fprintf(&b, " — id d'un objet de `%s`", f.Ref)
		}
		b.WriteString("\n")
	}
	names := make([]string, 0, len(vocabs))
	for v := range vocabs {
		names = append(names, v)
	}
	sort.Strings(names)
	for _, v := range names {
		fmt.Fprintf(&b, "\nCodes `%s` : ", v)
		for i, c := range Vocabularies[v].Codes {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%s (%s)", c[0], c[1])
		}
		b.WriteString("\n")
	}
	return b.String()
}

func clonePivot(p *models.SarPivot) *models.SarPivot {
	raw, _ := json.Marshal(p)
	var out models.SarPivot
	_ = json.Unmarshal(raw, &out)
	return &out
}

func (ag *Agent) prov(sources []string, explanation string) models.SarProvenance {
	at := ag.now()
	p := models.SarProvenance{Kind: models.SarProvAI, Explanation: explanation, RunID: ag.RunID, At: &at}
	for _, s := range sources {
		p.Sources = append(p.Sources, models.SarSourceRef{SourceID: s})
	}
	return p
}

// applyAIOps applies model ops on a copy; diagnostics are returned for repair.
func (ag *Agent) applyAIOps(p *models.SarPivot, ops []AIOp, prefixes []string) (*models.SarPivot, []models.SarIssue) {
	work := clonePivot(p)
	ids := ag.sourceIDs()
	var diags []models.SarIssue
	for i, o := range ops {
		if len(o.Sources) == 0 && o.Op != "remove" {
			diags = append(diags, models.SarIssue{Path: o.Path, Code: "AI_SOURCE_MISSING", Message: fmt.Sprintf("op %d : chaque valeur doit citer une source", i), Severity: "error", Layer: "agent"})
			continue
		}
		unknown := []string{}
		for _, s := range o.Sources {
			if !ids[s] && !strings.HasPrefix(s, "pivot/") {
				unknown = append(unknown, s)
			}
		}
		if len(unknown) > 0 {
			diags = append(diags, models.SarIssue{Path: o.Path, Code: "AI_SOURCE_UNKNOWN", Message: "sources inconnues : " + strings.Join(unknown, ", "), Severity: "error", Layer: "agent"})
			continue
		}
		if strings.TrimSpace(o.Explanation) == "" {
			diags = append(diags, models.SarIssue{Path: o.Path, Code: "AI_EXPLANATION_MISSING", Message: "justification manquante", Severity: "error", Layer: "agent"})
			continue
		}
		patch, err := o.Patch()
		if err != nil {
			diags = append(diags, models.SarIssue{Path: o.Path, Code: "AI_VALUE", Message: err.Error(), Severity: "error", Layer: "agent"})
			continue
		}
		if _, err := ApplyOps(work, Exts{}, []models.SarPatchOp{patch}, ag.prov(o.Sources, o.Explanation), Policy{Prefixes: prefixes}); err != nil {
			var pe PatchError
			if errors.As(err, &pe) {
				diags = append(diags, models.SarIssue{Path: pe.Path, Code: pe.Code, Message: pe.Message, Severity: "error", Layer: "agent"})
			} else {
				diags = append(diags, models.SarIssue{Path: o.Path, Code: "AI_PATCH", Message: err.Error(), Severity: "error", Layer: "agent"})
			}
		}
	}
	for _, is := range ValidatePivot(work) {
		if is.Severity != "error" {
			continue
		}
		for _, pre := range prefixes {
			if strings.HasPrefix(is.Path, pre) {
				diags = append(diags, is)
				break
			}
		}
	}
	return work, diags
}

// RunPivotStep runs one step with bounded repairs. The pivot changes only
// when an attempt validates.
func (ag *Agent) RunPivotStep(ctx context.Context, p *models.SarPivot, st Step, remarks []string) (models.SarRunStep, error) {
	res := models.SarRunStep{Name: "pivot:" + st.Name, Status: "running"}
	var previous string
	var diags []models.SarIssue
	for attempt := 1; attempt <= ag.attempts(); attempt++ {
		prompt, err := render("pivot_step.md", map[string]any{"Step": st.Name, "Instructions": st.Instructions, "Prefixes": st.Prefixes, "Fields": fieldsText(st.Prefixes), "Pivot": CompactPivot(p), "Diagnostics": diags, "Previous": previous, "Remarks": remarks, "Sources": SourcesText(ag.Sources, 400000)})
		if err != nil {
			return res, err
		}
		res.Attempts = attempt
		var out AIPivotOutput
		raw, err := ag.call(ctx, res.Name, attempt, prompt, &out)
		if errors.Is(err, ErrBudget) {
			res.Status = "budget_exhausted"
			return res, nil
		}
		if err != nil && raw == "" {
			res.Status, res.Explanation = "failed", err.Error()
			return res, err
		}
		previous = raw
		if err != nil {
			diags = []models.SarIssue{{Path: "response", Code: "AI_JSON_INVALID", Message: err.Error(), Severity: "error", Layer: "agent"}}
			continue
		}
		work, d := ag.applyAIOps(p, out.Ops, st.Prefixes)
		ag.trace("validation", res.Name, attempt, d)
		if len(d) == 0 {
			*p = *work
			res.Status, res.Issues = "completed", nil
			if len(out.Missing) > 0 {
				res.Explanation = "Manques signalés : " + strings.Join(out.Missing, " ; ")
			}
			return res, nil
		}
		diags, res.Issues = d, d
	}
	res.Status = "failed"
	return res, nil
}

// ReviewPivot asks for a global review and re-runs the targeted steps once.
func (ag *Agent) ReviewPivot(ctx context.Context, p *models.SarPivot) ([]models.SarRunStep, error) {
	prompt, err := render("review.md", map[string]any{"Pivot": CompactPivot(p), "Sources": SourcesText(ag.Sources, 400000)})
	if err != nil {
		return nil, err
	}
	review := models.SarRunStep{Name: "pivot:review", Status: "completed", Attempts: 1}
	var out AIReviewOutput
	if _, err := ag.call(ctx, review.Name, 1, prompt, &out); err != nil {
		if errors.Is(err, ErrBudget) {
			review.Status = "budget_exhausted"
			return []models.SarRunStep{review}, nil
		}
		review.Status, review.Explanation = "failed", err.Error()
		return []models.SarRunStep{review}, nil
	}
	byStep := map[string][]string{}
	for _, f := range out.Findings {
		byStep[f.Step] = append(byStep[f.Step], f.Path+" : "+f.Message)
		review.Issues = append(review.Issues, models.SarIssue{Path: f.Path, Code: "REVIEW_FINDING", Message: f.Message, Severity: "warning", Layer: "review"})
	}
	steps := []models.SarRunStep{review}
	for _, st := range PivotSteps {
		if remarks := byStep[st.Name]; len(remarks) > 0 {
			r, err := ag.RunPivotStep(ctx, p, st, remarks)
			r.Name += ":rework"
			steps = append(steps, r)
			if err != nil {
				return steps, err
			}
		}
	}
	return steps, nil
}

// Observation feeds the per-format memory.
type Observation struct {
	Code      string
	Path      string
	Cause     string
	Fix       string
	Corrected bool
}

func issueKey(i models.SarIssue) string { return i.Code + "|" + NormalizePivotPath(i.Path) }

// NormalizePivotPath removes indexes and object IDs for deduplication.
func NormalizePivotPath(path string) string {
	var b strings.Builder
	skip := false
	for _, r := range path {
		switch {
		case r == '[':
			skip = true
		case r == ']':
			skip = false
		case !skip:
			b.WriteRune(r)
		}
	}
	p := b.String()
	if i := strings.IndexByte(p, '#'); i >= 0 {
		p = p[:i]
	}
	if strings.HasPrefix(p, "pivot/") {
		p = "pivot/" + Pattern(p)
	}
	return p
}

func extFieldsText(a Adapter) string {
	var b strings.Builder
	s := a.Schema()
	for _, f := range a.Extension() {
		if !f.AI {
			continue
		}
		fmt.Fprintf(&b, "\n### `%s` (%s, portée %s, %d..%d) — %s\n", f.Key, f.Kind, f.Scope, f.Min, f.Max, f.Label)
		if f.Instructions != "" {
			b.WriteString(f.Instructions + "\n")
		}
		if l := s.CodeLists[f.CodeList]; l != nil {
			for _, c := range l.Codes {
				fmt.Fprintf(&b, "- %s : %s\n", c.Value, c.Label)
			}
		}
	}
	return b.String()
}

func aiExtCurrent(a Adapter, ext models.SarExtension) string {
	ai := map[string]bool{}
	for _, f := range a.Extension() {
		if f.AI || f.Computed {
			ai[f.Key] = true
		}
	}
	out := map[string]any{}
	for k, v := range ext {
		name, _, _ := strings.Cut(k, "#")
		if ai[name] {
			entry := map[string]any{"values": v.Values}
			if v.P != nil {
				entry["provenance"] = v.P.Kind
			}
			out[k] = entry
		}
	}
	raw, _ := json.MarshalIndent(out, "", " ")
	return string(raw)
}

func (ag *Agent) applyExt(a Adapter, in Input, out AIExtOutput) (models.SarExtension, []models.SarIssue) {
	key := a.Descriptor().Key
	exts := Exts{key: in.Ext}
	if exts[key] == nil {
		exts[key] = models.SarExtension{}
	}
	ids := ag.sourceIDs()
	objects := map[string]bool{}
	for _, x := range in.Pivot.Operations {
		objects[x.ID] = true
	}
	for _, x := range in.Pivot.Parties {
		objects[x.ID] = true
	}
	for _, x := range in.Pivot.Instruments {
		objects[x.ID] = true
	}
	for _, x := range in.Pivot.Attachments {
		objects[x.ID] = true
	}
	specs := map[string]ExtField{}
	for _, f := range a.Extension() {
		specs[f.Key] = f
	}
	var diags []models.SarIssue
	for _, v := range out.Values {
		path := "extension/" + key + "/" + ExtKey(v.Key, v.ObjectID)
		f, ok := specs[v.Key]
		bad := func(code, msg string) {
			diags = append(diags, models.SarIssue{Path: path, Code: code, Message: msg, Severity: "error", Layer: "agent", Edit: path})
		}
		switch {
		case !ok || !f.AI:
			bad("AI_EXT_FIELD", "champ non modifiable par le modèle : "+v.Key)
			continue
		case f.Scope != "report" && !objects[v.ObjectID]:
			bad("AI_EXT_OBJECT", "object_id inconnu dans le pivot : "+v.ObjectID)
			continue
		case f.Scope == "report" && v.ObjectID != "":
			bad("AI_EXT_OBJECT", "champ de portée report : object_id interdit")
			continue
		case strings.TrimSpace(v.Explanation) == "":
			bad("AI_EXPLANATION_MISSING", "justification manquante")
			continue
		}
		unknown := []string{}
		for i, s := range v.Sources {
			if ref, ok := pivotRef(in.Pivot, s); ok {
				v.Sources[i] = ref
			} else if !ids[s] {
				unknown = append(unknown, s)
			}
		}
		if len(unknown) > 0 || len(v.Sources) == 0 {
			bad("AI_SOURCE_UNKNOWN", "sources absentes ou inconnues : "+strings.Join(unknown, ", ")+" ; citez un chemin du pivot (pivot/operations/<id>, pivot/parties/<id>, pivot/suspicion) ou un identifiant de source du case")
			continue
		}
		valid := true
		for _, val := range v.Values {
			if f.CodeList != "" && !a.Schema().HasCode(f.CodeList, val) {
				bad("AI_EXT_CODE", fmt.Sprintf("%q n'est pas un code de %s", val, f.CodeList))
				valid = false
			}
		}
		if !valid {
			continue
		}
		raw, _ := json.Marshal(v.Values)
		_, err := ApplyOps(&models.SarPivot{}, exts, []models.SarPatchOp{{Op: "set", Path: path, Value: raw}}, ag.prov(v.Sources, v.Explanation), Policy{})
		if err != nil {
			bad("AI_EXT_PATCH", err.Error())
		}
	}
	return exts[key], diags
}

// pivotRef normalizes a pivot citation: a pivot path, a bare object ID or a
// section name ("suspicion", "analysis").
func pivotRef(p *models.SarPivot, s string) (string, bool) {
	if strings.HasPrefix(s, "pivot/") {
		return s, true
	}
	switch s {
	case "suspicion", "analysis", "pivot":
		return "pivot/suspicion", true
	}
	for coll, ids := range map[string][]string{"operations": opIDs(p), "parties": partyIDs(p), "instruments": instrumentIDs(p), "attachments": attachmentIDs(p), "links": linkIDs(p)} {
		for _, id := range ids {
			if id == s {
				return "pivot/" + coll + "/" + id, true
			}
		}
	}
	return "", false
}

func opIDs(p *models.SarPivot) (out []string) {
	for _, x := range p.Operations {
		out = append(out, x.ID)
	}
	return
}
func partyIDs(p *models.SarPivot) (out []string) {
	for _, x := range p.Parties {
		out = append(out, x.ID)
	}
	return
}
func instrumentIDs(p *models.SarPivot) (out []string) {
	for _, x := range p.Instruments {
		out = append(out, x.ID)
	}
	return
}
func attachmentIDs(p *models.SarPivot) (out []string) {
	for _, x := range p.Attachments {
		out = append(out, x.ID)
	}
	return
}
func linkIDs(p *models.SarPivot) (out []string) {
	for _, x := range p.Links {
		out = append(out, x.ID)
	}
	return
}

// extFixable keeps the issues the extension step can act on: those whose
// edit target is an extension value, or rule/extension issues not caused by
// a pivot fact (a pivot edit target means the analyst must fix a fact).
func extFixable(key string, issues []models.SarIssue) []models.SarIssue {
	var out []models.SarIssue
	for _, i := range issues {
		if i.Severity != "error" || strings.HasPrefix(i.Edit, "pivot/") || strings.HasPrefix(i.Edit, "settings/") || strings.HasPrefix(i.Edit, "configuration/") {
			continue
		}
		if strings.HasPrefix(i.Edit, "extension/"+key) || i.Layer == "rule" || i.Layer == "extension" {
			out = append(out, i)
		}
	}
	return out
}

// FillExtension fills the format-specific values with validator feedback.
func (ag *Agent) FillExtension(ctx context.Context, a Adapter, in Input, memory []models.SarMemoryEntry) (models.SarExtension, Evaluation, []Observation, models.SarRunStep) {
	key := a.Descriptor().Key
	step := models.SarRunStep{Name: "ext:" + key, Status: "running"}
	ev := Evaluate(a, in)
	in.Ext = ev.Ext
	var observations []Observation
	var agentDiags, agentDiagsBefore []models.SarIssue
	var prevFixable map[string]bool
	firstIssues := map[string]models.SarIssue{}
	for attempt := 1; attempt <= ag.attempts(); attempt++ {
		fixable := extFixable(key, ev.Issues)
		missingAI := false
		for _, f := range a.Extension() {
			if f.AI && f.Scope == "report" && f.Min > 0 && len(in.Ext[f.Key].Values) == 0 {
				missingAI = true
			}
		}
		if attempt > 1 && len(fixable) == 0 && len(agentDiags) == 0 || attempt == 1 && !missingAI && len(fixable) == 0 {
			step.Status = "completed"
			break
		}
		for _, i := range fixable {
			if _, ok := firstIssues[issueKey(i)]; !ok {
				firstIssues[issueKey(i)] = i
			}
		}
		prompt, err := render("extension.md", map[string]any{"Format": a.Descriptor().Label, "Instructions": a.Instructions(), "Fields": extFieldsText(a), "Current": aiExtCurrent(a, in.Ext), "Memory": memory, "Issues": append(fixable, agentDiags...), "Pivot": CompactPivot(in.Pivot)})
		if err != nil {
			step.Status, step.Explanation = "failed", err.Error()
			break
		}
		step.Attempts = attempt
		var out AIExtOutput
		_, err = ag.call(ctx, step.Name, attempt, prompt, &out)
		if errors.Is(err, ErrBudget) {
			step.Status = "budget_exhausted"
			break
		}
		if err != nil {
			agentDiags = []models.SarIssue{{Path: "response", Code: "AI_JSON_INVALID", Message: err.Error(), Severity: "error", Layer: "agent"}}
			continue
		}
		before := in.Ext
		if prevFixable == nil {
			prevFixable = map[string]bool{}
			for _, i := range fixable {
				prevFixable[issueKey(i)] = true
			}
			agentDiagsBefore = agentDiags
		}
		next, diags := ag.applyExt(a, in, out)
		agentDiags = diags
		in.Ext = next
		ev = Evaluate(a, in)
		in.Ext = ev.Ext
		now := map[string]bool{}
		for _, i := range extFixable(key, ev.Issues) {
			now[issueKey(i)] = true
		}
		progress := len(diags) < len(agentDiagsBefore) || len(now) < len(prevFixable)
		prevFixable, agentDiagsBefore = now, diags
		for k, i := range firstIssues {
			if !now[k] {
				observations = append(observations, Observation{Code: i.Code, Path: NormalizePivotPath(i.Path), Cause: i.Message, Fix: describeChange(before, in.Ext), Corrected: true})
				delete(firstIssues, k)
			}
		}
		step.Status = "completed"
		if attempt > 1 && !progress && len(diags) == 0 {
			// No improvement: the remaining issues need the analyst.
			break
		}
	}
	for _, i := range extFixable(key, ev.Issues) {
		observations = append(observations, Observation{Code: i.Code, Path: NormalizePivotPath(i.Path), Cause: i.Message})
	}
	for _, d := range agentDiags {
		observations = append(observations, Observation{Code: d.Code, Path: NormalizePivotPath(d.Path), Cause: d.Message})
	}
	step.Issues = append(append([]models.SarIssue{}, agentDiags...), ev.Issues...)
	// One observation per (code, normalized path) per run; a correction wins.
	seen := map[string]int{}
	var unique []Observation
	for _, o := range observations {
		k := o.Code + "|" + o.Path
		if i, ok := seen[k]; ok {
			if o.Corrected && !unique[i].Corrected {
				unique[i] = o
			}
			continue
		}
		seen[k] = len(unique)
		unique = append(unique, o)
	}
	observations = unique
	if ev.Blocking() > 0 && step.Status == "completed" {
		step.Status = "completed_with_issues"
	}
	return in.Ext, ev, observations, step
}

func describeChange(before, after models.SarExtension) string {
	var parts []string
	keys := make([]string, 0, len(after))
	for k := range after {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if strings.Join(before[k].Values, ",") != strings.Join(after[k].Values, ",") {
			parts = append(parts, fmt.Sprintf("%s = %s", k, strings.Join(after[k].Values, ", ")))
		}
	}
	if len(parts) == 0 {
		return "corrigé par le recalcul du backend"
	}
	out := strings.Join(parts, " ; ")
	if len(out) > 300 {
		out = out[:300] + "…"
	}
	return out
}

// Chat turns an analyst message into a patch proposal.
func (ag *Agent) Chat(ctx context.Context, a Adapter, in Input, issues []models.SarIssue, message string, history []models.SarChatMessage) (AIChatOutput, []models.SarPatchOp, []models.SarIssue, error) {
	var fields strings.Builder
	for _, f := range a.Extension() {
		if f.Scope != "settings" {
			fmt.Fprintf(&fields, "- `%s` (%s, %s%s)\n", f.Key, f.Label, f.Scope, map[bool]string{true: ", calculé"}[f.Computed])
		}
	}
	prompt, err := render("chat.md", map[string]any{"Format": a.Descriptor().Label, "Key": a.Descriptor().Key, "Message": message, "History": history, "Issues": issues, "Fields": fields.String(), "Current": aiExtCurrent(a, in.Ext), "Pivot": CompactPivot(in.Pivot)})
	if err != nil {
		return AIChatOutput{}, nil, nil, err
	}
	var out AIChatOutput
	if _, err := ag.call(ctx, "chat", 1, prompt, &out); err != nil {
		return out, nil, nil, err
	}
	var ops []models.SarPatchOp
	var diags []models.SarIssue
	for _, o := range out.Ops {
		p, err := o.Patch()
		if err != nil {
			diags = append(diags, models.SarIssue{Path: o.Path, Code: "AI_VALUE", Message: err.Error(), Severity: "error", Layer: "agent"})
			continue
		}
		ops = append(ops, p)
	}
	return out, ops, diags, nil
}
