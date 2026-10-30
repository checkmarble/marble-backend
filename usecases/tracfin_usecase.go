package usecases

import (
	"context"
	"encoding/json"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/tracfin"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"slices"
)

var (
	errTracfinSourceUnavailable = errors.Wrap(models.NotFoundError, "TRACFIN source unavailable")
	errTracfinLinkUnavailable   = errors.Wrap(models.BadParameterError, "TRACFIN configured link unavailable")
	errTracfinIndexUnavailable  = errors.Wrap(models.UnprocessableEntityError, "NAVIGATION_INDEX_UNAVAILABLE")
)

type tracfinCaseReader interface {
	GetCase(context.Context, string) (models.Case, error)
	ReadCasePivotObjects(context.Context, string) ([]models.PivotObject, error)
}
type tracfinExecutorFactory interface {
	NewExecutor() repositories.Executor
	NewClientDbExecutor(context.Context, uuid.UUID) (repositories.Executor, error)
}
type tracfinSourceRepository interface {
	GetTracfinDraft(context.Context, repositories.Executor, string, string) (models.TracfinDraft, error)
	GetTracfinDecisionSources(context.Context, repositories.Executor, string, string, []string) ([]models.TracfinDecisionSource, error)
}
type tracfinObjectReader interface {
	QueryIngestedObjectByUniqueField(context.Context, repositories.Executor, models.Table, string, string, ...string) ([]models.DataModelObject, error)
	ListIngestedObjects(context.Context, repositories.Executor, models.Table, models.ExplorationOptions, *string, int, ...string) ([]models.DataModelObject, error)
}
type TracfinUsecase struct {
	orgID                     uuid.UUID
	repository                tracfinSourceRepository
	cases                     tracfinCaseReader
	dataModelUsecase          ingestedDataReaderDataModelUsecase
	objects                   tracfinObjectReader
	executorFactory           tracfinExecutorFactory
	draftDependencies         *TracfinDraftDependencies
	configurationDependencies *TracfinConfigurationDependencies
}

func NewTracfinUsecase(orgID uuid.UUID, repository tracfinSourceRepository, cases tracfinCaseReader, dataModel ingestedDataReaderDataModelUsecase, objects tracfinObjectReader, executors tracfinExecutorFactory, drafts ...TracfinDraftDependencies) TracfinUsecase {
	uc := TracfinUsecase{orgID: orgID, repository: repository, cases: cases, dataModelUsecase: dataModel, objects: objects, executorFactory: executors}
	if len(drafts) > 0 {
		uc.draftDependencies = &drafts[0]
	}
	return uc
}

type TracfinRoot struct {
	Table    string   `json:"table"`
	RecordID string   `json:"record_id"`
	Origins  []string `json:"origins"` // case_manual_entity and decision_pivot remain distinct
}
type TracfinContext struct {
	Roots       []TracfinRoot              `json:"roots"`
	Diagnostics []models.TracfinDiagnostic `json:"diagnostics"`
}
type TracfinNavigationPage struct {
	Objects     []models.DataModelObject   `json:"objects"`
	HasNextPage bool                       `json:"has_next_page"`
	NextCursor  *string                    `json:"next_cursor,omitempty"`
	Diagnostics []models.TracfinDiagnostic `json:"diagnostics"`
}

func (uc TracfinUsecase) authorize(ctx context.Context, caseID string) (models.Case, error) {
	c, err := uc.cases.GetCase(ctx, caseID)
	if err != nil {
		return c, err
	}
	if c.Id != caseID || c.OrganizationId != uc.orgID {
		return models.Case{}, models.NotFoundError
	}
	return c, nil
}
func (uc TracfinUsecase) GetContext(ctx context.Context, caseID string) (TracfinContext, error) {
	c, err := uc.authorize(ctx, caseID)
	if err != nil {
		return TracfinContext{}, err
	}
	dm, err := uc.dataModelUsecase.GetDataModel(ctx, uc.orgID, models.DataModelReadOptions{}, false)
	if err != nil {
		return TracfinContext{}, err
	}
	return uc.caseContext(ctx, c, dm)
}

// caseContext lists the Person/Company roots of an already authorized case.
func (uc TracfinUsecase) caseContext(ctx context.Context, c models.Case, dm models.DataModel) (TracfinContext, error) {
	pivots, err := uc.cases.ReadCasePivotObjects(ctx, c.Id)
	if err != nil {
		return TracfinContext{}, err
	}
	out := TracfinContext{}
	positions := map[models.CaseEntityRef]int{}
	add := func(ref models.CaseEntityRef, origin string) {
		table, ok := dm.Tables[ref.TableName]
		if !ok || (table.SemanticType != models.SemanticTypePerson && table.SemanticType != models.SemanticTypeCompany) || ref.ObjectId == "" {
			return
		}
		i, ok := positions[ref]
		if !ok {
			i = len(out.Roots)
			positions[ref] = i
			out.Roots = append(out.Roots, TracfinRoot{Table: ref.TableName, RecordID: ref.ObjectId})
		}
		if !slices.Contains(out.Roots[i].Origins, origin) {
			out.Roots[i].Origins = append(out.Roots[i].Origins, origin)
		}
	}
	for _, entity := range c.Entities {
		add(entity.CaseEntityRef, "case_manual_entity")
	}
	for _, pivot := range pivots {
		if pivot.PivotType == models.PivotTypeObject && pivot.IsIngested {
			add(models.CaseEntityRef{TableName: pivot.PivotObjectName, ObjectId: pivot.PivotObjectId}, "decision_pivot")
		}
	}
	return out, nil
}
func (uc TracfinUsecase) draft(ctx context.Context, caseID string) (models.TracfinDraft, error) {
	d, _, err := uc.draftSources(ctx, caseID)
	return d, err
}

// draftSources reads the draft and a source loader under one case authorization.
func (uc TracfinUsecase) draftSources(ctx context.Context, caseID string) (models.TracfinDraft, *tracfinSourceLoader, error) {
	l, err := uc.sourceLoader(ctx, caseID)
	if err != nil {
		return models.TracfinDraft{}, nil, err
	}
	d, err := uc.repository.GetTracfinDraft(ctx, uc.executorFactory.NewExecutor(), uc.orgID.String(), caseID)
	if err == nil && (d.OrgID != uc.orgID.String() || d.CaseID != caseID) {
		return models.TracfinDraft{}, nil, models.NotFoundError
	}
	return d, l, err
}

// VerifySelection also serves editing: it is independent from applying proposals.
func (uc TracfinUsecase) VerifySelection(ctx context.Context, caseID string, selection models.TracfinSelection) (models.TracfinSelection, error) {
	d, err := uc.draft(ctx, caseID)
	if err != nil {
		return models.TracfinSelection{}, err
	}
	_, err = uc.verifySelection(ctx, caseID, d.Configuration, "selection", selection)
	return selection, err
}
func selectionSteps(config models.TracfinConfiguration, s models.TracfinSelection) (map[string]models.TracfinPathStep, error) {
	anchor, ok := tracfin.MappingForSelection(config, s)
	if !ok || !tracfin.CompatibleMapping(anchor, s) {
		return nil, models.BadParameterError
	}
	out := map[string]models.TracfinPathStep{}
	for _, m := range config.Mappings {
		if !tracfin.CompatibleMapping(m, s) {
			continue
		}
		parent := "root"
		for _, step := range m.Source.Path {
			if step.ID == "" || step.ID == "root" || step.ParentID != parent || step.Table == "" || step.Link == "" || (step.Direction != "forward" && step.Direction != "inverse") {
				return nil, models.BadParameterError
			}
			if old, ok := out[step.ID]; ok && (old.ParentID != step.ParentID || old.Table != step.Table || old.Link != step.Link || old.Direction != step.Direction) {
				return nil, models.BadParameterError
			}
			out[step.ID] = step
			parent = step.ID
		}
	}
	return out, nil
}
func (uc TracfinUsecase) readObject(ctx context.Context, dm models.DataModel, table, id string) (tracfin.SourceRecord, error) {
	t, ok := dm.Tables[table]
	if !ok {
		return tracfin.SourceRecord{}, errTracfinSourceUnavailable
	}
	exec, err := uc.executorFactory.NewClientDbExecutor(ctx, uc.orgID)
	if err != nil {
		return tracfin.SourceRecord{}, err
	}
	rows, err := uc.objects.QueryIngestedObjectByUniqueField(ctx, exec, t, id, "object_id")
	if err != nil {
		return tracfin.SourceRecord{}, err
	}
	if len(rows) != 1 || rows[0].Data["object_id"] != id {
		return tracfin.SourceRecord{}, errTracfinSourceUnavailable
	}
	return tracfin.SourceRecord{Table: table, RecordID: id, Data: rows[0].Data}, nil
}

// linkFields validates the configured named edge in the current org datamodel.
// Inverse collections require the exact current navigation option and valid index.
func linkFields(dm models.DataModel, parentTable string, step models.TracfinPathStep) (string, string, *models.NavigationOption, error) {
	parent, ok := dm.Tables[parentTable]
	if !ok {
		return "", "", nil, errTracfinSourceUnavailable
	}
	target, ok := dm.Tables[step.Table]
	if !ok {
		return "", "", nil, errTracfinSourceUnavailable
	}
	switch step.Direction {
	case "forward":
		link, ok := parent.LinksToSingle[step.Link]
		if !ok || link.ChildTableName != parentTable || link.ParentTableName != step.Table {
			return "", "", nil, errTracfinLinkUnavailable
		}
		return link.ChildFieldName, link.ParentFieldName, nil, nil
	case "inverse":
		link, ok := target.LinksToSingle[step.Link]
		if !ok || link.ParentTableName != parentTable || link.ChildTableName != step.Table {
			return "", "", nil, errTracfinLinkUnavailable
		}
		for _, nav := range parent.NavigationOptions {
			if nav.SourceFieldName == link.ParentFieldName && nav.TargetTableName == step.Table && nav.FilterFieldName == link.ChildFieldName && nav.Status == models.IndexStatusValid {
				return link.ParentFieldName, link.ChildFieldName, &nav, nil
			}
		}
		return "", "", nil, errTracfinIndexUnavailable
	}
	return "", "", nil, models.BadParameterError
}

// tracfinSourceLoader reads selected sources for one request. The case is
// authorized once, its roots and data model are read at most once, and a
// prefill over unchanged selections reuses the sources it already read.
type tracfinSourceLoader struct {
	uc      TracfinUsecase
	c       models.Case
	dm      *models.DataModel
	roots   map[models.CaseEntityRef]bool
	prefill map[string]tracfin.SourceSet
}

func (uc TracfinUsecase) sourceLoader(ctx context.Context, caseID string) (*tracfinSourceLoader, error) {
	c, err := uc.authorize(ctx, caseID)
	if err != nil {
		return nil, err
	}
	return &tracfinSourceLoader{uc: uc, c: c, prefill: map[string]tracfin.SourceSet{}}, nil
}

func (l *tracfinSourceLoader) dataModel(ctx context.Context) (models.DataModel, error) {
	if l.dm == nil {
		dm, err := l.uc.dataModelUsecase.GetDataModel(ctx, l.uc.orgID, models.DataModelReadOptions{IncludeNavigationOptions: true}, false)
		if err != nil {
			return models.DataModel{}, err
		}
		l.dm = &dm
	}
	return *l.dm, nil
}

func (l *tracfinSourceLoader) caseRoot(ctx context.Context, root models.TracfinPathStep) (bool, error) {
	if l.roots == nil {
		dm, err := l.dataModel(ctx)
		if err != nil {
			return false, err
		}
		cc, err := l.uc.caseContext(ctx, l.c, dm)
		if err != nil {
			return false, err
		}
		l.roots = map[models.CaseEntityRef]bool{}
		for _, r := range cc.Roots {
			l.roots[models.CaseEntityRef{TableName: r.Table, ObjectId: r.RecordID}] = true
		}
	}
	return l.roots[models.CaseEntityRef{TableName: root.Table, ObjectId: root.RecordID}], nil
}

func (uc TracfinUsecase) verifySelection(ctx context.Context, caseID string, config models.TracfinConfiguration, blockID string, s models.TracfinSelection) (tracfin.SourceSet, error) {
	l, err := uc.sourceLoader(ctx, caseID)
	if err != nil {
		return tracfin.SourceSet{}, err
	}
	return l.load(ctx, config, blockID, s, false)
}

// load verifies a selection against the configuration and current sources.
// Selection edits and navigation are strict. Prefill (stale) reads persisted
// selections that a configuration or source change may have made obsolete: it
// reports them on their block instead of failing the whole draft. Technical
// errors still fail.
func (l *tracfinSourceLoader) load(ctx context.Context, config models.TracfinConfiguration, blockID string, s models.TracfinSelection, stale bool) (tracfin.SourceSet, error) {
	uc := l.uc
	out := tracfin.SourceSet{Records: map[tracfin.SourceKey]tracfin.SourceRecord{}, Decisions: map[string]models.TracfinDecisionSource{}}
	report := func(code string) {
		out.Diagnostics = append(out.Diagnostics, models.TracfinDiagnostic{Code: code, Layer: "sources", Severity: "warning", Message: code, Field: &models.TracfinFieldRef{BlockID: blockID}})
	}
	anchor, ok := tracfin.MappingForSelection(config, s)
	if !ok && stale {
		// ResolvePrefill reports MAPPING_UNAVAILABLE for this block.
		return out, nil
	}
	if !ok || !tracfin.CompatibleMapping(anchor, s) {
		if stale {
			report("MAPPING_UNAVAILABLE")
			return out, nil
		}
		return out, models.BadParameterError
	}
	if s.DecisionID != "" {
		if len(s.Bindings) > 0 || s.Root.RecordID != "" {
			return out, models.BadParameterError
		}
		rows, err := uc.repository.GetTracfinDecisionSources(ctx, uc.executorFactory.NewExecutor(), uc.orgID.String(), l.c.Id, []string{s.DecisionID})
		if err != nil {
			return out, err
		}
		if len(rows) != 1 || rows[0].DecisionID != s.DecisionID || rows[0].HistoricalType != s.HistoricalType {
			return out, errTracfinSourceUnavailable
		}
		out.Decisions[s.DecisionID] = rows[0]
		return out, nil
	}
	allowed, err := selectionSteps(config, s)
	if err != nil {
		if stale {
			report("MAPPING_UNAVAILABLE")
			return out, nil
		}
		return out, err
	}
	if s.Root.ID != "root" || s.Root.ParentID != "" || s.Root.Link != "" || s.Root.Direction != "" || len(s.Bindings) > 20 {
		return out, models.BadParameterError
	}
	found, err := l.caseRoot(ctx, s.Root)
	if err != nil {
		return out, err
	}
	if !found {
		return out, errTracfinSourceUnavailable
	}
	dm, err := l.dataModel(ctx)
	if err != nil {
		return out, err
	}
	root, err := uc.readObject(ctx, dm, s.Root.Table, s.Root.RecordID)
	if err != nil {
		return out, err
	}
	out.Records[tracfin.SourceKey{BlockID: blockID, BindingID: "root"}] = root
	seen := map[string]bool{"root": true}
	unavailable := map[string]bool{}
	skip := func(bindingID, code string) {
		unavailable[bindingID] = true
		report(code)
	}
	skipUnavailable := func(bindingID string, err error) bool {
		code := tracfinAvailabilityCode(err)
		if !stale || code == "" {
			return false
		}
		skip(bindingID, code)
		return true
	}
	for _, binding := range s.Bindings {
		if seen[binding.ID] || binding.RecordID == "" {
			return out, models.BadParameterError
		}
		seen[binding.ID] = true
		step, ok := allowed[binding.ID]
		if !ok || binding.ParentID != step.ParentID || binding.Table != step.Table || binding.Link != step.Link || binding.Direction != step.Direction {
			if stale {
				skip(binding.ID, "SELECTION_STALE")
				continue
			}
			return out, models.BadParameterError
		}
		parent, ok := out.Records[tracfin.SourceKey{BlockID: blockID, BindingID: binding.ParentID}]
		if !ok {
			if stale && unavailable[binding.ParentID] {
				unavailable[binding.ID] = true
				continue
			}
			return out, models.BadParameterError
		}
		parentField, childField, _, err := linkFields(dm, parent.Table, step)
		if err != nil {
			if skipUnavailable(binding.ID, err) {
				continue
			}
			return out, err
		}
		child, err := uc.readObject(ctx, dm, binding.Table, binding.RecordID)
		if err != nil {
			if skipUnavailable(binding.ID, err) {
				continue
			}
			return out, err
		}
		// Exact strings are the supported relation keys. No float conversion or null equality.
		p, pok := parent.Data[parentField].(string)
		c, cok := child.Data[childField].(string)
		if !pok || !cok || p == "" || p != c {
			if stale {
				skip(binding.ID, "SELECTION_PARENT_CHANGED")
				continue
			}
			return out, errors.Wrap(models.BadParameterError, "selected object does not belong to selected parent")
		}
		out.Records[tracfin.SourceKey{BlockID: blockID, BindingID: binding.ID}] = child
	}
	return out, nil
}
func (uc TracfinUsecase) Navigate(ctx context.Context, caseID string, s models.TracfinSelection, stepID string, cursor *string, limit int) (TracfinNavigationPage, error) {
	d, l, err := uc.draftSources(ctx, caseID)
	if err != nil {
		return TracfinNavigationPage{}, err
	}
	return l.navigate(ctx, d.Configuration, s, stepID, cursor, limit)
}
func (uc TracfinUsecase) navigate(ctx context.Context, caseID string, config models.TracfinConfiguration, s models.TracfinSelection, stepID string, cursor *string, limit int) (TracfinNavigationPage, error) {
	l, err := uc.sourceLoader(ctx, caseID)
	if err != nil {
		return TracfinNavigationPage{}, err
	}
	return l.navigate(ctx, config, s, stepID, cursor, limit)
}
func (l *tracfinSourceLoader) navigate(ctx context.Context, config models.TracfinConfiguration, s models.TracfinSelection, stepID string, cursor *string, limit int) (TracfinNavigationPage, error) {
	uc := l.uc
	page := TracfinNavigationPage{}
	if limit < 1 || limit > 100 || s.DecisionID != "" {
		return page, models.BadParameterError
	}
	sources, err := l.load(ctx, config, "navigation", s, false)
	if err != nil {
		return page, err
	}
	steps, err := selectionSteps(config, s)
	if err != nil {
		return page, err
	}
	step, ok := steps[stepID]
	if !ok {
		return page, models.BadParameterError
	}
	parent, ok := sources.Records[tracfin.SourceKey{BlockID: "navigation", BindingID: step.ParentID}]
	if !ok {
		return page, models.BadParameterError
	}
	dm, err := l.dataModel(ctx)
	if err != nil {
		return page, err
	}
	sourceField, filterField, nav, err := linkFields(dm, parent.Table, step)
	if errors.Is(err, errTracfinIndexUnavailable) {
		page.Diagnostics = []models.TracfinDiagnostic{{Code: "NAVIGATION_INDEX_UNAVAILABLE", Layer: "sources", Severity: "warning", Message: "The selected collection requires an available navigation index"}}
		return page, nil
	}
	if err != nil {
		return page, err
	}
	value, ok := parent.Data[sourceField].(string)
	if !ok || value == "" {
		return page, models.BadParameterError
	}
	exec, err := uc.executorFactory.NewClientDbExecutor(ctx, uc.orgID)
	if err != nil {
		return page, err
	}
	if nav == nil {
		if cursor != nil {
			return page, models.BadParameterError
		}
		page.Objects, err = uc.objects.QueryIngestedObjectByUniqueField(ctx, exec, dm.Tables[step.Table], value, filterField)
		if len(page.Objects) > 1 {
			return page, errors.Wrap(models.UnprocessableEntityError, "TRACFIN forward link is not unique")
		}
		return page, err
	}
	rows, err := uc.objects.ListIngestedObjects(ctx, exec, dm.Tables[step.Table], models.ExplorationOptions{SourceTableName: parent.Table, FilterFieldName: filterField, FilterFieldValue: models.NewStringOrNumberFromString(value), OrderingFieldName: nav.OrderingFieldName}, cursor, limit+1)
	if err != nil {
		return page, err
	}
	page.HasNextPage = len(rows) > limit
	page.Objects = rows[:min(limit, len(rows))]
	if page.HasNextPage {
		id, ok := page.Objects[len(page.Objects)-1].Data["object_id"].(string)
		if !ok || id == "" {
			return page, errors.New("navigation result has no object ID")
		}
		page.NextCursor = &id
	}
	return page, nil
}
func (uc TracfinUsecase) Prefill(ctx context.Context, caseID string) (tracfin.PrefillResult, error) {
	d, l, err := uc.draftSources(ctx, caseID)
	if err != nil {
		return tracfin.PrefillResult{}, err
	}
	return l.prefillDraft(ctx, d)
}

// prefillDraft reads the sources of every saved selection once per selection
// set, then resolves proposals for the current draft content.
func (l *tracfinSourceLoader) prefillDraft(ctx context.Context, d models.TracfinDraft) (tracfin.PrefillResult, error) {
	type selected struct {
		Block     string                  `json:"block"`
		Selection models.TracfinSelection `json:"selection"`
	}
	var selections []selected
	tracfin.WalkBlocks(d.Document.Root, func(b models.TracfinBlock) {
		if b.Selection != nil {
			selections = append(selections, selected{b.ID, *b.Selection})
		}
	})
	encoded, err := json.Marshal(selections)
	if err != nil {
		return tracfin.PrefillResult{}, err
	}
	key := tracfin.MappingVersion(d.Configuration) + ":" + string(encoded)
	sources, cached := l.prefill[key]
	if !cached {
		sources = tracfin.SourceSet{Records: map[tracfin.SourceKey]tracfin.SourceRecord{}, Decisions: map[string]models.TracfinDecisionSource{}}
		for _, s := range selections {
			loaded, err := l.load(ctx, d.Configuration, s.Block, s.Selection, true)
			if err != nil {
				code := tracfinAvailabilityCode(err)
				if code == "" {
					return tracfin.PrefillResult{}, err
				}
				sources.Diagnostics = append(sources.Diagnostics, models.TracfinDiagnostic{Code: code, Layer: "sources", Severity: "warning", Message: code, Field: &models.TracfinFieldRef{BlockID: s.Block}})
			}
			sources.Diagnostics = append(sources.Diagnostics, loaded.Diagnostics...)
			for k, v := range loaded.Records {
				sources.Records[k] = v
			}
			for k, v := range loaded.Decisions {
				sources.Decisions[k] = v
			}
		}
		l.prefill[key] = sources
	}
	return tracfin.ResolvePrefill(d.Configuration, d, sources), nil
}

func tracfinAvailabilityCode(err error) string {
	switch {
	case errors.Is(err, errTracfinSourceUnavailable):
		return "SOURCE_UNAVAILABLE"
	case errors.Is(err, errTracfinIndexUnavailable):
		return "NAVIGATION_INDEX_UNAVAILABLE"
	case errors.Is(err, errTracfinLinkUnavailable):
		return "LINK_UNAVAILABLE"
	}
	return ""
}
