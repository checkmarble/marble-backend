package usecases

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/ai_agent"
	"github.com/checkmarble/marble-backend/usecases/sar"
	"github.com/checkmarble/marble-backend/usecases/sar/format"
	_ "github.com/checkmarble/marble-backend/usecases/sar/formats/goaml"
	_ "github.com/checkmarble/marble-backend/usecases/sar/formats/tracfin"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/invopop/jsonschema"
)

// SarUsecase orchestrates the multi-format suspicious activity reports of a
// case: one pivot, one report per format, the agent loop, memory and chat.
type SarUsecase struct {
	orgID        uuid.UUID
	repo         repositories.SarRepository
	cases        sarCaseAccess
	executors    tracfinExecutorFactory
	transactions tracfinDraftTransactions
	files        tracfinCaseFileRepository
	blobs        sarBlobReader
	llm          sar.LLM
	enqueue      func(context.Context, repositories.Transaction, models.SarRunArgs) error
	now          func() time.Time
}

type sarCaseAccess interface {
	AuthorizeCase(ctx context.Context, caseID string) (models.Case, error)
	CollectAIContext(ctx context.Context, caseID string) (models.TracfinAIContext, error)
}

type sarBlobReader interface {
	GetBlob(context.Context, string, string, ...repositories.GetBlobOption) (models.Blob, error)
}

func (uc TracfinUsecase) AuthorizeCase(ctx context.Context, caseID string) (models.Case, error) {
	return uc.authorize(ctx, caseID)
}

// sarLLM adapts the TRACFIN model configuration (AI Studio / Vertex).
type sarLLM struct{ ai *ai_agent.AiAgentUsecase }

func (l sarLLM) Complete(ctx context.Context, system, prompt string, schema *jsonschema.Schema) (string, error) {
	r, err := l.ai.TracfinAIComplete(ctx, system, prompt, *schema)
	return r.Text, err
}

func (uc SarUsecase) actor(ctx context.Context) (string, error) {
	creds, ok := utils.CredentialsFromCtx(ctx)
	if !ok || creds.OrganizationId != uc.orgID || creds.ActorIdentity.UserId == "" {
		return "", models.ForbiddenError
	}
	return string(creds.ActorIdentity.UserId), nil
}

func (uc SarUsecase) access(ctx context.Context, caseID string) (string, error) {
	actor, err := uc.actor(ctx)
	if err != nil {
		return "", err
	}
	c, err := uc.cases.AuthorizeCase(ctx, caseID)
	if err != nil {
		return "", err
	}
	if c.OrganizationId != uc.orgID {
		return "", models.NotFoundError
	}
	return actor, nil
}

func (uc SarUsecase) org() string { return uc.orgID.String() }

func (uc SarUsecase) clock() time.Time {
	if uc.now != nil {
		return uc.now()
	}
	return time.Now().UTC()
}

// ---------------------------------------------------------------- formats & configuration

type SarFormatInfo struct {
	sar.Descriptor
	Extension []sar.ExtField `json:"extension"`
}

func (uc SarUsecase) ListFormats(ctx context.Context) ([]SarFormatInfo, error) {
	if _, err := uc.actor(ctx); err != nil {
		return nil, err
	}
	var out []SarFormatInfo
	for _, a := range sar.Adapters() {
		out = append(out, SarFormatInfo{Descriptor: a.Descriptor(), Extension: a.Extension()})
	}
	return out, nil
}

func (uc SarUsecase) GetConfiguration(ctx context.Context) (models.SarConfiguration, error) {
	if _, err := uc.actor(ctx); err != nil {
		return models.SarConfiguration{}, err
	}
	return uc.repo.GetSarConfiguration(ctx, uc.executors.NewExecutor(), uc.org())
}

type SarConfigurationInput struct {
	ExpectedRevision int64                          `json:"expected_revision"`
	Mappings         []models.SarTableMapping       `json:"mappings"`
	Settings         map[string]map[string][]string `json:"settings"`
	Rates            map[string]string              `json:"rates"`
}

func validateSarConfiguration(in SarConfigurationInput) error {
	entities := map[string]string{"operation": "operations", "person": "parties", "entity": "parties", "instrument": "instruments"}
	extra := map[string]bool{"owner_id": true, "street": true, "postal_code": true, "city": true, "country": true}
	for _, m := range in.Mappings {
		coll, ok := entities[m.Entity]
		if !ok || m.Table == "" || m.ID == "" {
			return errors.Wrapf(models.BadParameterError, "mapping %s: id, table and entity (operation, person, entity, instrument) are required", m.ID)
		}
		if m.AmountUnit != "" && m.AmountUnit != "major" && m.AmountUnit != "minor" {
			return errors.Wrap(models.BadParameterError, "amount_unit must be major or minor")
		}
		if m.Timezone != "" {
			if _, err := time.LoadLocation(m.Timezone); err != nil {
				return errors.Wrap(models.BadParameterError, "unknown timezone "+m.Timezone)
			}
		}
		for target := range m.Fields {
			if _, ok := sar.FieldByPath(coll + "/*/" + target); !ok && !(coll != "operations" && extra[target]) {
				return errors.Wrapf(models.BadParameterError, "mapping %s: unknown pivot field %s", m.ID, target)
			}
		}
	}
	for key, values := range in.Settings {
		a, ok := sar.Get(key)
		if !ok {
			return errors.Wrap(models.BadParameterError, "unknown format "+key)
		}
		known := map[string]bool{}
		for _, f := range a.Extension() {
			if f.Scope == "settings" {
				known[f.Key] = true
			}
		}
		for k := range values {
			if !known[k] {
				return errors.Wrapf(models.BadParameterError, "unknown setting %s for %s", k, key)
			}
		}
	}
	for pair, rate := range in.Rates {
		from, to, ok := strings.Cut(pair, "/")
		if !ok || len(from) != 3 || len(to) != 3 || sar.DecimalCmp(rate, "0") <= 0 {
			return errors.Wrap(models.BadParameterError, "rates are FROM/TO: positive decimal")
		}
	}
	return nil
}

func (uc SarUsecase) PutConfiguration(ctx context.Context, in SarConfigurationInput) (models.SarConfiguration, error) {
	actor, err := uc.actor(ctx)
	if err != nil {
		return models.SarConfiguration{}, err
	}
	if err := validateSarConfiguration(in); err != nil {
		return models.SarConfiguration{}, err
	}
	c := models.SarConfiguration{OrgID: uc.org(), Mappings: in.Mappings, Settings: in.Settings, Rates: in.Rates, UpdatedAt: uc.clock(), UpdatedBy: actor}
	if err := uc.repo.SaveSarConfiguration(ctx, uc.executors.NewExecutor(), c, in.ExpectedRevision); err != nil {
		return models.SarConfiguration{}, err
	}
	return uc.repo.GetSarConfiguration(ctx, uc.executors.NewExecutor(), uc.org())
}

// ---------------------------------------------------------------- pivot

type SarPivotView struct {
	Pivot        models.SarPivot      `json:"pivot"`
	Issues       []models.SarIssue    `json:"issues"`
	Fields       []sar.FieldSpec      `json:"fields"`
	Vocabularies map[string]sar.Vocab `json:"vocabularies"`
}

func (uc SarUsecase) GetPivot(ctx context.Context, caseID string) (SarPivotView, error) {
	if _, err := uc.access(ctx, caseID); err != nil {
		return SarPivotView{}, err
	}
	p, _, err := uc.repo.GetSarPivot(ctx, uc.executors.NewExecutor(), uc.org(), caseID, false)
	if err != nil {
		return SarPivotView{}, err
	}
	return SarPivotView{Pivot: p, Issues: sar.ValidatePivot(&p), Fields: sar.PivotFields, Vocabularies: sar.Vocabularies}, nil
}

type SarPatchInput struct {
	ExpectedRevision int64               `json:"expected_revision"`
	Ops              []models.SarPatchOp `json:"ops"`
	Summary          string              `json:"summary,omitempty"`
}

// PatchPivot applies analyst edits; every report of the case is re-evaluated.
func (uc SarUsecase) PatchPivot(ctx context.Context, caseID string, in SarPatchInput) (SarPivotView, error) {
	actor, err := uc.access(ctx, caseID)
	if err != nil {
		return SarPivotView{}, err
	}
	for _, op := range in.Ops {
		if !strings.HasPrefix(op.Path, "pivot/") {
			return SarPivotView{}, errors.Wrap(models.BadParameterError, "pivot patches only; use the report endpoint for extensions")
		}
	}
	at := uc.clock()
	prov := models.SarProvenance{Kind: models.SarProvManual, ActorID: actor, At: &at}
	err = uc.transactions.Transaction(ctx, func(tx repositories.Transaction) error {
		p, _, err := uc.repo.GetSarPivot(ctx, tx, uc.org(), caseID, true)
		if err != nil {
			return err
		}
		if p.Revision != in.ExpectedRevision {
			return errors.Wrap(models.ConflictError, "pivot revision changed")
		}
		if _, err := sar.ApplyOps(&p, sar.Exts{}, in.Ops, prov, sar.Policy{AllowManualOverwrite: true, AllowAnalysis: true}); err != nil {
			return errors.Wrap(models.BadParameterError, err.Error())
		}
		p.Validated = nil
		return uc.savePivot(ctx, tx, &p, in.ExpectedRevision, actor, "manual", "", "", firstNonEmpty(in.Summary, "édition manuelle"), in.Ops)
	})
	if err != nil {
		return SarPivotView{}, err
	}
	if err := uc.refreshReports(ctx, caseID, actor); err != nil {
		return SarPivotView{}, err
	}
	return uc.GetPivot(ctx, caseID)
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}

func (uc SarUsecase) savePivot(ctx context.Context, exec repositories.Executor, p *models.SarPivot, expected int64, actor, origin, runID, chatID, summary string, ops []models.SarPatchOp) error {
	p.OrgID, p.UpdatedAt, p.UpdatedBy = uc.org(), uc.clock(), actor
	if err := uc.repo.SaveSarPivot(ctx, exec, *p, expected); err != nil {
		return err
	}
	p.Revision = expected + 1
	patch, _ := json.Marshal(ops)
	return uc.repo.AddSarRevision(ctx, exec, models.SarRevision{OrgID: uc.org(), CaseID: p.CaseID, Object: "pivot", Revision: p.Revision, Origin: origin, ActorID: actor, RunID: runID, ChatID: chatID, Summary: summary, Patch: patch}, p)
}

func (uc SarUsecase) ValidatePivotFacts(ctx context.Context, caseID string, expected int64) (SarPivotView, error) {
	actor, err := uc.access(ctx, caseID)
	if err != nil {
		return SarPivotView{}, err
	}
	err = uc.transactions.Transaction(ctx, func(tx repositories.Transaction) error {
		p, _, err := uc.repo.GetSarPivot(ctx, tx, uc.org(), caseID, true)
		if err != nil {
			return err
		}
		if p.Revision != expected || expected == 0 {
			return errors.Wrap(models.ConflictError, "pivot revision changed")
		}
		for _, i := range sar.ValidatePivot(&p) {
			if i.Severity == "error" {
				return errors.Wrap(models.UnprocessableEntityError, "pivot has blocking issues: "+i.Path+" "+i.Message)
			}
		}
		p.Validated = &models.SarValidation{Revision: expected + 1, ActorID: actor, At: uc.clock()}
		return uc.savePivot(ctx, tx, &p, expected, actor, "validation", "", "", "faits validés par l'analyste", nil)
	})
	if err != nil {
		return SarPivotView{}, err
	}
	return uc.GetPivot(ctx, caseID)
}

func (uc SarUsecase) ListRevisions(ctx context.Context, caseID, object string) ([]models.SarRevision, error) {
	if _, err := uc.access(ctx, caseID); err != nil {
		return nil, err
	}
	return uc.repo.ListSarRevisions(ctx, uc.executors.NewExecutor(), uc.org(), caseID, object)
}

// ---------------------------------------------------------------- runs

type SarRunInput struct {
	Mode    string   `json:"mode"` // pivot, reports, all
	Formats []string `json:"formats"`
}

func sarBudget(mode string, formats int) int {
	steps := len(sar.PivotSteps)
	pivot := 3*steps + 1 + 3*steps
	switch mode {
	case "pivot":
		return pivot
	case "reports":
		return 3 * formats
	}
	return pivot + 3*formats
}

func (uc SarUsecase) StartRun(ctx context.Context, caseID string, in SarRunInput) (models.SarRun, error) {
	actor, err := uc.access(ctx, caseID)
	if err != nil {
		return models.SarRun{}, err
	}
	if in.Mode == "" {
		in.Mode = "all"
	}
	if in.Mode != "pivot" && in.Mode != "reports" && in.Mode != "all" {
		return models.SarRun{}, errors.Wrap(models.BadParameterError, "mode is pivot, reports or all")
	}
	if in.Mode != "pivot" && len(in.Formats) == 0 {
		return models.SarRun{}, errors.Wrap(models.BadParameterError, "formats are required")
	}
	for _, f := range in.Formats {
		if _, ok := sar.Get(f); !ok {
			return models.SarRun{}, errors.Wrap(models.BadParameterError, "unknown format "+f)
		}
	}
	if uc.enqueue == nil || uc.llm == nil {
		return models.SarRun{}, errors.Wrap(models.UnprocessableEntityError, "SAR agent dependencies unavailable")
	}
	now := uc.clock()
	run := models.SarRun{ID: uuid.NewString(), OrgID: uc.org(), CaseID: caseID, ActorID: actor, Formats: in.Formats, Mode: in.Mode, Status: "queued", CallsBudget: sarBudget(in.Mode, len(in.Formats)), CreatedAt: now, UpdatedAt: now}
	err = uc.transactions.Transaction(ctx, func(tx repositories.Transaction) error {
		if err := uc.repo.CreateSarRun(ctx, tx, run); err != nil {
			return err
		}
		return uc.enqueue(ctx, tx, models.SarRunArgs{OrgID: uc.org(), CaseID: caseID, RunID: run.ID, ActorID: actor})
	})
	return run, err
}

func (uc SarUsecase) GetRun(ctx context.Context, caseID, runID string) (models.SarRun, error) {
	if _, err := uc.access(ctx, caseID); err != nil {
		return models.SarRun{}, err
	}
	return uc.repo.GetSarRun(ctx, uc.executors.NewExecutor(), uc.org(), caseID, runID)
}

// ExecuteRun is the worker side of a run (also called directly by tests).
func (uc SarUsecase) ExecuteRun(ctx context.Context, caseID, runID string) (models.SarRun, error) {
	actor, err := uc.access(ctx, caseID)
	if err != nil {
		return models.SarRun{}, err
	}
	exec := uc.executors.NewExecutor()
	run, err := uc.repo.GetSarRun(ctx, exec, uc.org(), caseID, runID)
	if err != nil {
		return run, err
	}
	if run.ActorID != actor {
		return run, models.ForbiddenError
	}
	if run.Status != "queued" {
		return run, nil
	}
	run.Status = "running"
	_ = uc.repo.SaveSarRun(ctx, exec, run)
	fail := func(e error) (models.SarRun, error) {
		run.Status, run.Error, run.UpdatedAt = "failed", e.Error(), uc.clock()
		_ = uc.repo.SaveSarRun(context.WithoutCancel(ctx), exec, run)
		return run, e
	}
	source, err := uc.cases.CollectAIContext(ctx, caseID)
	if err != nil {
		return fail(err)
	}
	cfg, err := uc.repo.GetSarConfiguration(ctx, exec, uc.org())
	if err != nil {
		return fail(err)
	}
	p, _, err := uc.repo.GetSarPivot(ctx, exec, uc.org(), caseID, false)
	if err != nil {
		return fail(err)
	}
	startRev := p.Revision
	trace, closeTrace := sarTrace(ctx, run.ID)
	defer closeTrace()
	ag := &sar.Agent{LLM: uc.llm, Budget: run.CallsBudget, RunID: run.ID, Now: uc.clock, Sources: source.Sources, Trace: trace}
	if run.Mode != "reports" {
		mapped := sar.MapSources(cfg, caseID, uc.org(), source.Sources)
		mergeMapped(&p, mapped.Pivot)
		for _, d := range mapped.Diagnostics {
			run.Steps = append(run.Steps, models.SarRunStep{Name: "mapping", Status: "warning", Issues: []models.SarIssue{d}})
		}
		for _, st := range sar.PivotSteps {
			res, err := ag.RunPivotStep(ctx, &p, st, nil)
			run.Steps = append(run.Steps, res)
			if err != nil {
				return fail(err)
			}
		}
		reviews, err := ag.ReviewPivot(ctx, &p)
		run.Steps = append(run.Steps, reviews...)
		if err != nil {
			return fail(err)
		}
		p.Validated = nil
		p.CaseID = caseID
		if err := uc.savePivot(ctx, exec, &p, startRev, actor, "agent", run.ID, "", "génération du pivot par l'agent", nil); err != nil {
			return fail(errors.Wrap(err, "pivot changed during the run"))
		}
	}
	for _, key := range run.Formats {
		a, _ := sar.Get(key)
		step, err := uc.generateReport(ctx, ag, a, &p, cfg, actor, run.ID)
		run.Steps = append(run.Steps, step)
		if err != nil {
			return fail(err)
		}
	}
	run.CallsUsed = ag.Used
	run.Status, run.UpdatedAt = "completed", uc.clock()
	if err := uc.repo.SaveSarRun(ctx, exec, run); err != nil {
		return run, err
	}
	return run, nil
}

// mergeMapped adds mapped objects missing from the pivot (by stable ID).
func mergeMapped(p *models.SarPivot, m models.SarPivot) {
	has := map[string]bool{}
	for _, x := range p.Parties {
		has[x.ID] = true
	}
	for _, x := range p.Instruments {
		has[x.ID] = true
	}
	for _, x := range p.Operations {
		has[x.ID] = true
	}
	for _, x := range p.Attachments {
		has[x.ID] = true
	}
	for _, x := range p.Links {
		has[x.ID] = true
	}
	for _, x := range m.Parties {
		if !has[x.ID] {
			p.Parties = append(p.Parties, x)
		}
	}
	for _, x := range m.Instruments {
		if !has[x.ID] {
			p.Instruments = append(p.Instruments, x)
		}
	}
	for _, x := range m.Links {
		if !has[x.ID] {
			p.Links = append(p.Links, x)
		}
	}
	for _, x := range m.Operations {
		if !has[x.ID] {
			p.Operations = append(p.Operations, x)
		}
	}
	for _, x := range m.Attachments {
		if !has[x.ID] {
			p.Attachments = append(p.Attachments, x)
		}
	}
}

func (uc SarUsecase) input(p *models.SarPivot, cfg models.SarConfiguration, key string, ext models.SarExtension) sar.Input {
	return sar.Input{Pivot: p, Ext: ext, Settings: cfg.Settings[key], Rates: cfg.Rates, Now: uc.clock()}
}

func (uc SarUsecase) findReport(ctx context.Context, exec repositories.Executor, caseID, key string) (models.SarReport, bool, error) {
	reports, err := uc.repo.ListSarReports(ctx, exec, uc.org(), caseID)
	if err != nil {
		return models.SarReport{}, false, err
	}
	for _, r := range reports {
		if r.FormatKey == key {
			return r, true, nil
		}
	}
	return models.SarReport{}, false, nil
}

func (uc SarUsecase) generateReport(ctx context.Context, ag *sar.Agent, a sar.Adapter, p *models.SarPivot, cfg models.SarConfiguration, actor, runID string) (models.SarRunStep, error) {
	exec := uc.executors.NewExecutor()
	key := a.Descriptor().Key
	rep, found, err := uc.findReport(ctx, exec, p.CaseID, key)
	if err != nil {
		return models.SarRunStep{Name: "ext:" + key, Status: "failed"}, err
	}
	expected := rep.Revision
	if !found {
		rep = models.SarReport{ID: uuid.NewString(), CaseID: p.CaseID, OrgID: uc.org(), FormatKey: key, Extension: models.SarExtension{}, CreatedAt: uc.clock()}
		expected = 0
	}
	memory, err := uc.repo.ListSarMemory(ctx, exec, uc.org(), key, 30)
	if err != nil {
		return models.SarRunStep{Name: "ext:" + key, Status: "failed"}, err
	}
	ext, ev, observations, step := ag.FillExtension(ctx, a, uc.input(p, cfg, key, rep.Extension), memory)
	rep.Extension, rep.ReportType, rep.Issues, rep.PivotRevision = ext, ev.ReportType, ev.Issues, p.Revision
	rep.Status = models.SarReportReady
	if ev.Blocking() > 0 {
		rep.Status = models.SarReportInvalid
	}
	rep.ApprovedRevision, rep.ApprovedBy = 0, ""
	if err := uc.saveReport(ctx, exec, &rep, expected, actor, "agent", runID, "", "génération par l'agent", nil); err != nil {
		return step, err
	}
	for _, o := range observations {
		if err := uc.repo.UpsertSarMemory(ctx, exec, models.SarMemoryEntry{OrgID: uc.org(), FormatKey: key, Code: o.Code, Path: o.Path, Cause: o.Cause, Fix: o.Fix, Corrected: map[bool]int{true: 1}[o.Corrected]}); err != nil {
			return step, err
		}
	}
	return step, nil
}

func (uc SarUsecase) saveReport(ctx context.Context, exec repositories.Executor, rep *models.SarReport, expected int64, actor, origin, runID, chatID, summary string, ops []models.SarPatchOp) error {
	rep.UpdatedAt, rep.UpdatedBy = uc.clock(), actor
	if err := uc.repo.SaveSarReport(ctx, exec, *rep, expected); err != nil {
		return err
	}
	rep.Revision = expected + 1
	patch, _ := json.Marshal(ops)
	return uc.repo.AddSarRevision(ctx, exec, models.SarRevision{OrgID: uc.org(), CaseID: rep.CaseID, Object: "report:" + rep.ID, Revision: rep.Revision, Origin: origin, ActorID: actor, RunID: runID, ChatID: chatID, Summary: summary, Patch: patch}, rep)
}

// refreshReports re-evaluates every report after a pivot change.
func (uc SarUsecase) refreshReports(ctx context.Context, caseID, actor string) error {
	exec := uc.executors.NewExecutor()
	p, _, err := uc.repo.GetSarPivot(ctx, exec, uc.org(), caseID, false)
	if err != nil {
		return err
	}
	cfg, err := uc.repo.GetSarConfiguration(ctx, exec, uc.org())
	if err != nil {
		return err
	}
	reports, err := uc.repo.ListSarReports(ctx, exec, uc.org(), caseID)
	if err != nil {
		return err
	}
	for _, rep := range reports {
		a, ok := sar.Get(rep.FormatKey)
		if !ok {
			continue
		}
		ev := sar.Evaluate(a, uc.input(&p, cfg, rep.FormatKey, rep.Extension))
		rep.Extension, rep.ReportType, rep.Issues, rep.PivotRevision = ev.Ext, ev.ReportType, ev.Issues, p.Revision
		rep.Status = map[bool]string{true: models.SarReportInvalid, false: models.SarReportReady}[ev.Blocking() > 0]
		rep.ApprovedRevision, rep.ApprovedBy = 0, ""
		if err := uc.saveReport(ctx, exec, &rep, rep.Revision, actor, "pivot_change", "", "", fmt.Sprintf("réévaluation après la révision %d du pivot", p.Revision), nil); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------- reports

func (uc SarUsecase) ListReports(ctx context.Context, caseID string) ([]models.SarReport, error) {
	if _, err := uc.access(ctx, caseID); err != nil {
		return nil, err
	}
	return uc.repo.ListSarReports(ctx, uc.executors.NewExecutor(), uc.org(), caseID)
}

func (uc SarUsecase) load(ctx context.Context, caseID, reportID string) (sar.Adapter, models.SarReport, models.SarPivot, models.SarConfiguration, error) {
	exec := uc.executors.NewExecutor()
	rep, err := uc.repo.GetSarReport(ctx, exec, uc.org(), caseID, reportID)
	if err != nil {
		return nil, rep, models.SarPivot{}, models.SarConfiguration{}, err
	}
	a, ok := sar.Get(rep.FormatKey)
	if !ok {
		return nil, rep, models.SarPivot{}, models.SarConfiguration{}, errors.Wrap(models.UnprocessableEntityError, "format no longer supported")
	}
	p, _, err := uc.repo.GetSarPivot(ctx, exec, uc.org(), caseID, false)
	if err != nil {
		return nil, rep, p, models.SarConfiguration{}, err
	}
	cfg, err := uc.repo.GetSarConfiguration(ctx, exec, uc.org())
	return a, rep, p, cfg, err
}

func (uc SarUsecase) GetReportView(ctx context.Context, caseID, reportID string) (sar.ReportView, error) {
	if _, err := uc.access(ctx, caseID); err != nil {
		return sar.ReportView{}, err
	}
	a, rep, p, cfg, err := uc.load(ctx, caseID, reportID)
	if err != nil {
		return sar.ReportView{}, err
	}
	return sar.BuildView(a, uc.input(&p, cfg, rep.FormatKey, rep.Extension), rep), nil
}

type SarReportPatchInput struct {
	ExpectedRevision      int64               `json:"expected_revision"`
	ExpectedPivotRevision int64               `json:"expected_pivot_revision"`
	Ops                   []models.SarPatchOp `json:"ops"`
	Summary               string              `json:"summary,omitempty"`
}

// PatchReport edits a report through its edit paths (pivot or extension).
func (uc SarUsecase) PatchReport(ctx context.Context, caseID, reportID string, in SarReportPatchInput) (sar.ReportView, error) {
	actor, err := uc.access(ctx, caseID)
	if err != nil {
		return sar.ReportView{}, err
	}
	at := uc.clock()
	prov := models.SarProvenance{Kind: models.SarProvManual, ActorID: actor, At: &at}
	if err := uc.applyReportOps(ctx, caseID, reportID, in.ExpectedRevision, in.ExpectedPivotRevision, in.Ops, prov, sar.Policy{AllowManualOverwrite: true, AllowAnalysis: true}, actor, "manual", "", firstNonEmpty(in.Summary, "édition manuelle")); err != nil {
		return sar.ReportView{}, err
	}
	return uc.GetReportView(ctx, caseID, reportID)
}

func (uc SarUsecase) applyReportOps(ctx context.Context, caseID, reportID string, expected, expectedPivot int64, ops []models.SarPatchOp, prov models.SarProvenance, policy sar.Policy, actor, origin, chatID, summary string) error {
	pivotChanged := false
	err := uc.transactions.Transaction(ctx, func(tx repositories.Transaction) error {
		rep, err := uc.repo.GetSarReport(ctx, tx, uc.org(), caseID, reportID)
		if err != nil {
			return err
		}
		if rep.Revision != expected {
			return errors.Wrap(models.ConflictError, "report revision changed")
		}
		p, _, err := uc.repo.GetSarPivot(ctx, tx, uc.org(), caseID, true)
		if err != nil {
			return err
		}
		if p.Revision != expectedPivot {
			return errors.Wrap(models.ConflictError, "pivot revision changed")
		}
		cfg, err := uc.repo.GetSarConfiguration(ctx, tx, uc.org())
		if err != nil {
			return err
		}
		exts := sar.Exts{rep.FormatKey: rep.Extension}
		for _, op := range ops {
			if strings.HasPrefix(op.Path, "extension/") && !strings.HasPrefix(op.Path, "extension/"+rep.FormatKey+"/") {
				return errors.Wrap(models.BadParameterError, "extension path of another format")
			}
			pivotChanged = pivotChanged || strings.HasPrefix(op.Path, "pivot/")
		}
		if _, err := sar.ApplyOps(&p, exts, ops, prov, policy); err != nil {
			return errors.Wrap(models.BadParameterError, err.Error())
		}
		if pivotChanged {
			p.Validated = nil
			var pivotOps []models.SarPatchOp
			for _, op := range ops {
				if strings.HasPrefix(op.Path, "pivot/") {
					pivotOps = append(pivotOps, op)
				}
			}
			if err := uc.savePivot(ctx, tx, &p, expectedPivot, actor, origin, "", chatID, summary, pivotOps); err != nil {
				return err
			}
		}
		a, _ := sar.Get(rep.FormatKey)
		ev := sar.Evaluate(a, uc.input(&p, cfg, rep.FormatKey, exts[rep.FormatKey]))
		rep.Extension, rep.ReportType, rep.Issues, rep.PivotRevision = ev.Ext, ev.ReportType, ev.Issues, p.Revision
		rep.Status = map[bool]string{true: models.SarReportInvalid, false: models.SarReportReady}[ev.Blocking() > 0]
		rep.ApprovedRevision, rep.ApprovedBy = 0, ""
		return uc.saveReport(ctx, tx, &rep, expected, actor, origin, "", chatID, summary, ops)
	})
	if err != nil || !pivotChanged {
		return err
	}
	return uc.refreshReports(ctx, caseID, actor)
}

func (uc SarUsecase) Approve(ctx context.Context, caseID, reportID string, expected int64) (sar.ReportView, error) {
	actor, err := uc.access(ctx, caseID)
	if err != nil {
		return sar.ReportView{}, err
	}
	view, err := uc.GetReportView(ctx, caseID, reportID)
	if err != nil {
		return view, err
	}
	if view.Report.Revision != expected {
		return view, errors.Wrap(models.ConflictError, "report revision changed")
	}
	if !view.Exportable {
		return view, errors.Wrap(models.UnprocessableEntityError, "report has blocking issues")
	}
	rep := view.Report
	rep.Status, rep.ApprovedRevision, rep.ApprovedBy = models.SarReportApproved, expected+1, actor
	if err := uc.saveReport(ctx, uc.executors.NewExecutor(), &rep, expected, actor, "approval", "", "", "relecture humaine approuvée", nil); err != nil {
		return view, err
	}
	return uc.GetReportView(ctx, caseID, reportID)
}

// Export projects the report with attachment contents, validates it again and
// packages it (XML with base64 pieces for TRACFIN, ZIP for goAML).
func (uc SarUsecase) Export(ctx context.Context, caseID, reportID string) (sar.Artifact, error) {
	if _, err := uc.access(ctx, caseID); err != nil {
		return sar.Artifact{}, err
	}
	a, rep, p, cfg, err := uc.load(ctx, caseID, reportID)
	if err != nil {
		return sar.Artifact{}, err
	}
	if rep.Status != models.SarReportApproved || rep.ApprovedRevision != rep.Revision || rep.PivotRevision != p.Revision {
		return sar.Artifact{}, errors.Wrap(models.UnprocessableEntityError, "the current revision must be approved before export")
	}
	in := uc.input(&p, cfg, rep.FormatKey, rep.Extension)
	in.Files = map[string]sar.AttachmentContent{}
	var contents []sar.AttachmentContent
	for _, at := range sar.SelectedAttachments(&p) {
		c, err := uc.readAttachment(ctx, caseID, at)
		if err != nil {
			return sar.Artifact{}, err
		}
		in.Files[at.ID] = c
		contents = append(contents, c)
	}
	ev := sar.Evaluate(a, in)
	if ev.Blocking() > 0 {
		return sar.Artifact{}, errors.Wrapf(models.UnprocessableEntityError, "report has %d blocking issues", ev.Blocking())
	}
	var docs [][]byte
	for _, d := range ev.Docs {
		b, err := format.XMLBytes(a.Schema(), d)
		if err != nil {
			return sar.Artifact{}, err
		}
		docs = append(docs, b)
	}
	in.Ext = ev.Ext
	return a.Package(docs, contents, in)
}

func (uc SarUsecase) readAttachment(ctx context.Context, caseID string, at *models.SarAttachment) (sar.AttachmentContent, error) {
	if uc.files == nil || uc.blobs == nil {
		return sar.AttachmentContent{}, errors.New("attachment storage unavailable")
	}
	f, err := uc.files.GetCaseFileById(ctx, uc.executors.NewExecutor(), at.FileID)
	if err != nil {
		return sar.AttachmentContent{}, err
	}
	if f.CaseId != caseID {
		return sar.AttachmentContent{}, errors.Wrap(models.NotFoundError, "attachment outside the case")
	}
	blob, err := uc.blobs.GetBlob(ctx, f.BucketName, f.FileReference)
	if err != nil {
		return sar.AttachmentContent{}, err
	}
	defer blob.ReadCloser.Close()
	data, err := io.ReadAll(io.LimitReader(blob.ReadCloser, 20<<20+1))
	if err != nil {
		return sar.AttachmentContent{}, err
	}
	if len(data) > 20<<20 {
		return sar.AttachmentContent{}, errors.Wrap(models.BadParameterError, "attachment exceeds 20 MiB")
	}
	h := sha256.Sum256(data)
	if want := at.SHA256.Val(); want != "" && want != hex.EncodeToString(h[:]) {
		return sar.AttachmentContent{}, errors.Wrap(models.ConflictError, "attachment content changed since it was read")
	}
	return sar.AttachmentContent{AttachmentID: at.ID, Name: firstNonEmpty(at.Name.Val(), f.FileName), Mime: at.Mime.Val(), Data: data}, nil
}

func (uc SarUsecase) ListMemory(ctx context.Context, formatKey string) ([]models.SarMemoryEntry, error) {
	if _, err := uc.actor(ctx); err != nil {
		return nil, err
	}
	return uc.repo.ListSarMemory(ctx, uc.executors.NewExecutor(), uc.org(), formatKey, 0)
}

// ---------------------------------------------------------------- chat

type SarChatInput struct {
	Message string `json:"message"`
	// Apply defaults to true: a valid proposal becomes a new revision at once.
	Apply *bool `json:"apply,omitempty"`
}

type SarChatResult struct {
	User      models.SarChatMessage `json:"user"`
	Assistant models.SarChatMessage `json:"assistant"`
	View      *sar.ReportView       `json:"view,omitempty"`
}

func (uc SarUsecase) ListChat(ctx context.Context, caseID, reportID string) ([]models.SarChatMessage, error) {
	if _, err := uc.access(ctx, caseID); err != nil {
		return nil, err
	}
	if _, err := uc.repo.GetSarReport(ctx, uc.executors.NewExecutor(), uc.org(), caseID, reportID); err != nil {
		return nil, err
	}
	return uc.repo.ListSarChatMessages(ctx, uc.executors.NewExecutor(), uc.org(), reportID)
}

func (uc SarUsecase) PostChat(ctx context.Context, caseID, reportID string, in SarChatInput) (SarChatResult, error) {
	actor, err := uc.access(ctx, caseID)
	if err != nil {
		return SarChatResult{}, err
	}
	if strings.TrimSpace(in.Message) == "" || len(in.Message) > 8000 {
		return SarChatResult{}, errors.Wrap(models.BadParameterError, "message is required (8000 characters max)")
	}
	if uc.llm == nil {
		return SarChatResult{}, errors.Wrap(models.UnprocessableEntityError, "SAR agent dependencies unavailable")
	}
	a, rep, p, cfg, err := uc.load(ctx, caseID, reportID)
	if err != nil {
		return SarChatResult{}, err
	}
	exec := uc.executors.NewExecutor()
	history, err := uc.repo.ListSarChatMessages(ctx, exec, uc.org(), reportID)
	if err != nil {
		return SarChatResult{}, err
	}
	now := uc.clock()
	user := models.SarChatMessage{ID: uuid.NewString(), OrgID: uc.org(), CaseID: caseID, ReportID: reportID, Role: "user", Content: in.Message, ActorID: actor, CreatedAt: now, PivotRevision: p.Revision, ReportRevision: rep.Revision}
	if err := uc.repo.AddSarChatMessage(ctx, exec, user); err != nil {
		return SarChatResult{}, err
	}
	trace, closeTrace := sarTrace(ctx, "chat-"+user.ID)
	defer closeTrace()
	ag := &sar.Agent{LLM: uc.llm, Budget: 1, Now: uc.clock, Trace: trace}
	inp := uc.input(&p, cfg, rep.FormatKey, rep.Extension)
	ev := sar.Evaluate(a, inp)
	out, ops, diags, err := ag.Chat(ctx, a, inp, ev.Issues, in.Message, history)
	asst := models.SarChatMessage{ID: uuid.NewString(), OrgID: uc.org(), CaseID: caseID, ReportID: reportID, Role: "assistant", ActorID: actor, CreatedAt: uc.clock(), Patch: ops, Issues: diags}
	switch {
	case err != nil:
		asst.Status, asst.Content = "failed", "La proposition n'a pas pu être générée : "+err.Error()
	case out.Refused:
		asst.Status, asst.Content, asst.Patch = "refused", out.Reply, nil
	case len(diags) > 0 || len(ops) == 0:
		asst.Status, asst.Content = "rejected", out.Reply
	default:
		asst.Content = out.Reply
		// Dry run on copies: the patch must apply and keep the report valid
		// or improve it before it becomes a revision.
		trial := p
		exts := sar.Exts{rep.FormatKey: maps.Clone(rep.Extension)}
		at := uc.clock()
		prov := models.SarProvenance{Kind: models.SarProvAI, ChatID: user.ID, ActorID: actor, At: &at, Explanation: "demande de l'analyste : " + truncateText(in.Message, 200)}
		if _, perr := sar.ApplyOps(&trial, exts, ops, prov, sar.Policy{AllowManualOverwrite: true, AllowAnalysis: mentionsAnalysis(in.Message)}); perr != nil {
			asst.Status = "rejected"
			asst.Issues = append(asst.Issues, models.SarIssue{Path: "patch", Code: "CHAT_PATCH_INVALID", Message: perr.Error(), Severity: "error", Layer: "agent"})
			break
		}
		after := sar.Evaluate(a, uc.input(&trial, cfg, rep.FormatKey, exts[rep.FormatKey]))
		asst.Issues = after.Issues
		asst.Status = "proposed"
		if in.Apply == nil || *in.Apply {
			if aerr := uc.applyReportOps(ctx, caseID, reportID, rep.Revision, p.Revision, ops, prov, sar.Policy{AllowManualOverwrite: true, AllowAnalysis: mentionsAnalysis(in.Message)}, actor, "chat", user.ID, "correction par chat : "+truncateText(in.Message, 120)); aerr != nil {
				asst.Status = "failed"
				asst.Issues = append(asst.Issues, models.SarIssue{Path: "patch", Code: "CHAT_APPLY_FAILED", Message: aerr.Error(), Severity: "error", Layer: "agent"})
			} else {
				asst.Status = "applied"
			}
		}
	}
	asst.ReportRevision, asst.PivotRevision = rep.Revision, p.Revision
	if asst.Status == "applied" {
		if latest, e := uc.repo.GetSarReport(ctx, exec, uc.org(), caseID, reportID); e == nil {
			asst.ReportRevision, asst.PivotRevision = latest.Revision, latest.PivotRevision
		}
	}
	if err := uc.repo.AddSarChatMessage(ctx, exec, asst); err != nil {
		return SarChatResult{}, err
	}
	view, err := uc.GetReportView(ctx, caseID, reportID)
	if err != nil {
		return SarChatResult{User: user, Assistant: asst}, err
	}
	return SarChatResult{User: user, Assistant: asst, View: &view}, nil
}

// ApplyChat applies a proposal made with apply=false.
func (uc SarUsecase) ApplyChat(ctx context.Context, caseID, reportID, messageID string) (SarChatResult, error) {
	actor, err := uc.access(ctx, caseID)
	if err != nil {
		return SarChatResult{}, err
	}
	msgs, err := uc.ListChat(ctx, caseID, reportID)
	if err != nil {
		return SarChatResult{}, err
	}
	i := slices.IndexFunc(msgs, func(m models.SarChatMessage) bool { return m.ID == messageID && m.Role == "assistant" })
	if i < 0 {
		return SarChatResult{}, models.NotFoundError
	}
	m := msgs[i]
	if m.Status != "proposed" {
		return SarChatResult{}, errors.Wrap(models.ConflictError, "only a proposed patch can be applied")
	}
	if m.ReportRevision == 0 {
		return SarChatResult{}, errors.Wrap(models.ConflictError, "proposal without revision")
	}
	var request string
	if i > 0 {
		request = msgs[i-1].Content
	}
	at := uc.clock()
	prov := models.SarProvenance{Kind: models.SarProvAI, ChatID: m.ID, ActorID: actor, At: &at, Explanation: "demande de l'analyste : " + truncateText(request, 200)}
	if err := uc.applyReportOps(ctx, caseID, reportID, m.ReportRevision, m.PivotRevision, m.Patch, prov, sar.Policy{AllowManualOverwrite: true, AllowAnalysis: mentionsAnalysis(request)}, actor, "chat", m.ID, "correction par chat appliquée"); err != nil {
		return SarChatResult{}, err
	}
	m.Status = "applied"
	if err := uc.repo.UpdateSarChatMessage(ctx, uc.executors.NewExecutor(), m); err != nil {
		return SarChatResult{}, err
	}
	view, err := uc.GetReportView(ctx, caseID, reportID)
	return SarChatResult{Assistant: m, View: &view}, err
}

func mentionsAnalysis(s string) bool {
	return strings.Contains(strings.ToLower(s), "analyse")
}

func truncateText(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
